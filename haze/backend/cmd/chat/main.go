package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cotton-msg/haze/backend/internal/handler"
	"github.com/cotton-msg/haze/backend/internal/repository"
	"github.com/cotton-msg/haze/backend/internal/service"
	"github.com/cotton-msg/haze/backend/pkg/auth"
	"github.com/cotton-msg/haze/backend/pkg/config"
	"github.com/cotton-msg/haze/backend/pkg/metrics"
	"github.com/cotton-msg/haze/backend/pkg/middleware"
	"github.com/cotton-msg/haze/backend/pkg/queue"
	"github.com/cotton-msg/haze/backend/pkg/ws"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	_ "github.com/lib/pq"
)

type Config struct {
	Port        string
	DatabaseURL string
	JWTSecret   string
	LogLevel    string

	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	RedisKeyPrefix string
	QueueStream    string
	UnfurlStream   string

	SearchURL       string
	NotificationURL string
	SearchToken     string

	ServiceName string
}

func loadConfig() *Config {
	cfg, err := config.Load(envStr("CONFIG_FILE", "configs/chat.yaml"))
	if err != nil {
		log.Printf("config warning: %v", err)
	}
	return &Config{
		Port:        envStr("CHAT_PORT", cfg.Str("server.port", "8082")),
		DatabaseURL: envStr("DATABASE_URL", cfg.Str("database.url", "postgres://haze:haze@localhost:5432/haze?sslmode=disable")),
		JWTSecret:   envStr("JWT_SECRET", cfg.Str("jwt.access_secret", "change-me-in-production")),
		LogLevel:    envStr("CHAT_LOG_LEVEL", cfg.Str("server.log_level", "info")),

		RedisAddr:      envStr("REDIS_ADDR", cfg.Str("redis.address", "localhost:6379")),
		RedisPassword:  envStr("REDIS_PASSWORD", cfg.Str("redis.password", "")),
		RedisDB:        envInt("REDIS_DB", cfg.Int("redis.db", 1)),
		RedisKeyPrefix: "haze:ws",
		QueueStream:    envStr("QUEUE_STREAM", cfg.Str("queue.stream", "haze:notif")),
		UnfurlStream:   envStr("UNFURL_STREAM", cfg.Str("queue.unfurl_stream", "haze:unfurl")),

		SearchURL:       envStr("SERVICE_SEARCH", cfg.Str("services.search", "http://localhost:8087")),
		NotificationURL: envStr("SERVICE_NOTIFICATION", cfg.Str("services.notification", "http://localhost:8086")),
		SearchToken:     envStr("SEARCH_INTERNAL_TOKEN", cfg.Str("services.search_token", "")),

		ServiceName: "chat",
	}
}

func main() {
	cfg := loadConfig()

	zapCfg := zap.NewProductionConfig()
	zapCfg.Level.SetLevel(zap.InfoLevel)
	logger, err := zapCfg.Build()
	if err != nil {
		log.Fatalf("failed to build logger: %v", err)
	}
	defer logger.Sync()

	stdLog := log.New(os.Stdout, "[chat] ", log.LstdFlags)

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		stdLog.Printf("warning: database not reachable: %v (service will start anyway)", err)
	}

	chatRepo := repository.NewChatRepository(db)
	msgRepo := repository.NewMessageRepository(db)
	stickerRepo := repository.NewStickerRepository(db)
	reactionRepo := repository.NewReactionRepository(db)

	hub := ws.NewHub(logger)

	// Кластерная рассылка через Redis PubSub (fallback — локальный хаб).
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	var presenceSvc *service.PresenceService
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := rdb.Ping(ctx).Err(); err == nil {
		hub.WithClusterBackend(ws.NewRedisClusterBackend(rdb, cfg.RedisKeyPrefix))
		presenceSvc = service.NewPresenceService(rdb, cfg.RedisKeyPrefix)
		hub.SetPresenceHook(presenceSvc)
		stdLog.Printf("ws cluster backend: redis (%s)", cfg.RedisAddr)
	} else {
		stdLog.Printf("redis unavailable (%v) — ws cluster backend disabled", err)
	}
	cancel()

	chatSvc := service.NewChatService(chatRepo, msgRepo, hub)
	if err := rdb.Ping(context.Background()).Err(); err == nil {
		chatSvc.SetMessageCache(service.NewRedisMessageCache(rdb, cfg.RedisKeyPrefix))
		chatSvc.SetChatListCache(service.NewRedisChatListCache(rdb, cfg.RedisKeyPrefix))
	}
	if presenceSvc != nil {
		chatSvc.SetPresenceService(presenceSvc)
	}
	chatSvc.SetIndexer(service.NewHTTPIndexer(cfg.SearchURL).SetInternalToken(cfg.SearchToken))

	// Push-нотификации: через Redis Stream при доступном Redis, иначе HTTP-хук.
	chatSvc.SetPushNotifier(service.NewHTTPPushNotifier(cfg.NotificationURL))
	if err := rdb.Ping(context.Background()).Err(); err == nil {
		chatSvc.SetPushNotifier(service.NewRedisPushNotifier(queue.NewProducer(rdb, cfg.QueueStream)))
		stdLog.Printf("notification queue: redis stream (%s)", cfg.QueueStream)

		// Unfurl: ссылки в сообщениях → OG-карточки.
		chatSvc.SetUnfurlProducer(newUnfurlProducer(queue.NewProducer(rdb, cfg.UnfurlStream)))
		go runUnfurlConsumer(chatSvc, rdb, cfg.UnfurlStream)
		stdLog.Printf("unfurl queue: redis stream (%s)", cfg.UnfurlStream)
	}
	chatHandler := handler.NewChatHandler(chatSvc)
	reactionHandler := handler.NewReactionHandler(reactionRepo, stickerRepo, chatRepo, msgRepo)
	topicHandler := handler.NewTopicHandler(repository.NewTopicRepository(db), chatRepo)
	folderHandler := handler.NewFolderHandler(repository.NewFolderRepository(db))

	jwtCfg := &auth.Config{
		AccessSecret:  cfg.JWTSecret,
		RefreshSecret: cfg.JWTSecret,
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    720 * time.Hour,
	}

	wsHandler := ws.NewWSHandler(hub, logger)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"chat"}`))
	})
	r.Handle("/metrics", metrics.Handler())

	r.Route("/chat", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(auth.JWTMiddleware(jwtCfg))
			r.Post("/create", chatHandler.CreateChat)
			r.Get("/list", chatHandler.ListChats)
			r.Get("/saved", chatHandler.SavedChat)
			r.Get("/sync", chatHandler.SyncChat)
			r.Get("/{id}", chatHandler.GetChat)
			r.Get("/{id}/members", chatHandler.GetMembers)
			r.Get("/users/{userId}/presence", chatHandler.Presence)
			r.Post("/{id}/message", chatHandler.SendMessage)
			r.Post("/{id}/read", chatHandler.MarkRead)
			r.Post("/{id}/typing", chatHandler.Typing)
			r.Get("/{id}/messages", chatHandler.GetMessages)
			r.Put("/message/{msgId}", chatHandler.EditMessage)
			r.Delete("/message/{msgId}", chatHandler.DeleteMessage)
			r.Post("/{id}/members", chatHandler.AddMember)
			r.Delete("/{id}/members/{userId}", chatHandler.RemoveMember)
			r.Post("/message/{msgId}/reaction", reactionHandler.AddReaction)
			r.Delete("/message/{msgId}/reaction/{emoji}", reactionHandler.RemoveReaction)
			r.Get("/message/{msgId}/reactions", reactionHandler.GetReactions)
			r.Post("/topics", topicHandler.CreateTopic)
			r.Get("/topics", topicHandler.ListTopics)
			r.Put("/topics/{id}", topicHandler.UpdateTopic)
			r.Delete("/topics/{id}", topicHandler.DeleteTopic)
			r.Post("/folders", folderHandler.Create)
			r.Get("/folders", folderHandler.List)
			r.Put("/folders/{id}", folderHandler.Update)
			r.Delete("/folders/{id}", folderHandler.Delete)
			r.Post("/folders/{id}/chats", folderHandler.AddChat)
			r.Delete("/folders/{id}/chats/{chatId}", folderHandler.RemoveChat)
		})
	})

	r.Get("/stickers/packs", reactionHandler.GetStickerPacks)
	r.Get("/stickers/{packId}", reactionHandler.GetStickers)
	r.Group(func(r chi.Router) {
		r.Use(auth.JWTMiddleware(jwtCfg))
		r.Post("/stickers/packs", reactionHandler.CreateStickerPack)
		r.Put("/stickers/packs/{packId}", reactionHandler.UpdateStickerPack)
		r.Delete("/stickers/packs/{packId}", reactionHandler.DeleteStickerPack)
		r.Post("/stickers/packs/{packId}/stickers", reactionHandler.AddSticker)
		r.Delete("/stickers/{stickerId}", reactionHandler.DeleteSticker)
	})

	r.With(auth.WSJWTMiddleware(jwtCfg)).Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		wsHandler.ServeWS(w, r)
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	stdLog.Printf("Chat service starting on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		var n int
		if _, err := fmt.Sscanf(v, "%d", &n); err == nil {
			return n
		}
	}
	return def
}

// unfurlProducer адаптирует queue.Producer под интерфейс UnfurlProducer сервиса.
type unfurlProducer struct {
	p *queue.Producer
}

func newUnfurlProducer(p *queue.Producer) *unfurlProducer {
	return &unfurlProducer{p: p}
}

func (u *unfurlProducer) EnqueueUnfurl(messageID, url string) error {
	body, _ := json.Marshal(map[string]string{"message_id": messageID, "url": url})
	return u.p.Enqueue(queue.Message{Type: "unfurl", Target: messageID, Body: body})
}

// runUnfurlConsumer запускает воркер разбора OG-метаданных из Redis Stream.
func runUnfurlConsumer(svc *service.ChatService, rdb *redis.Client, stream string) {
	consumer := queue.NewConsumer(rdb, stream, "unfurl", func(m queue.Message) error {
		if m.Type != "unfurl" {
			return nil
		}
		var body struct {
			MessageID string `json:"message_id"`
			URL       string `json:"url"`
		}
		if err := json.Unmarshal(m.Body, &body); err != nil || body.MessageID == "" || body.URL == "" {
			return fmt.Errorf("bad unfurl job payload")
		}
		return svc.UnfurlJob(body.MessageID, body.URL)
	})
	if err := consumer.Run(context.Background(), time.Second); err != nil {
		log.Printf("unfurl consumer stopped: %v", err)
	}
}
