package main

import (
	"context"
	"database/sql"
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

	TurnURL      string
	TurnUsername string
	TurnPassword string
}

func loadConfig() *Config {
	raw, _ := config.Load("configs/call.yaml")
	return &Config{
		Port:        raw.EnvStr("CALL_PORT", "server.port", "8084"),
		DatabaseURL: raw.EnvStr("DATABASE_URL", "database.url", "postgres://haze:haze@localhost:5432/haze?sslmode=disable"),
		JWTSecret:   raw.EnvStr("JWT_SECRET", "auth.jwt_secret", "change-me-in-production"),
		LogLevel:    raw.EnvStr("CALL_LOG_LEVEL", "server.log_level", "info"),

		RedisAddr:      raw.EnvStr("REDIS_ADDR", "redis.address", "localhost:6379"),
		RedisPassword:  raw.EnvStr("REDIS_PASSWORD", "redis.password", ""),
		RedisDB:        raw.EnvInt("REDIS_DB", "redis.db", 1),
		RedisKeyPrefix: "haze:ws",

		TurnURL:      raw.EnvStr("TURN_URL", "turn.url", ""),
		TurnUsername: raw.EnvStr("TURN_USERNAME", "turn.username", ""),
		TurnPassword: raw.EnvStr("TURN_PASSWORD", "turn.password", ""),
	}
}

func main() {
	cfg := loadConfig()

	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("failed to build logger: %v", err)
	}
	defer logger.Sync()

	stdLog := log.New(os.Stdout, "[call] ", log.LstdFlags)

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to connect to database", zap.Error(err))
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		stdLog.Printf("warning: database not reachable: %v", err)
	}

	callRepo := repository.NewCallRepository(db)

	// События звонков публикуются в общий кластерный канал WS-хаба чата,
	// чтобы у клиента был единый WebSocket (chat/ws).
	var hub service.CallHub
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := rdb.Ping(ctx).Err(); err == nil {
		cancel()
		hub = newCallHub(rdb, cfg.RedisKeyPrefix)
		stdLog.Printf("call events: redis pubsub channel (%s)", cfg.RedisKeyPrefix)
	} else {
		cancel()
		stdLog.Printf("redis unavailable (%v) — call events will not be delivered", err)
	}

	callSvc := service.NewCallService(callRepo, hub)
	callSvc.SetIceServers(buildIceServers(cfg))
	callHandler := handler.NewCallHandler(callSvc)

	jwtCfg := &auth.Config{
		AccessSecret:  cfg.JWTSecret,
		RefreshSecret: cfg.JWTSecret,
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    720 * time.Hour,
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"call"}`))
	})
	r.Handle("/metrics", metrics.Handler())

	r.Route("/call", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(auth.JWTMiddleware(jwtCfg))
			r.Post("/start", callHandler.StartCall)
			r.Post("/{id}/answer", callHandler.AnswerCall)
			r.Post("/{id}/reject", callHandler.RejectCall)
			r.Post("/{id}/end", callHandler.EndCall)
			r.Post("/{id}/signal", callHandler.Signaling)
			r.Get("/history", callHandler.GetHistory)
			r.Get("/ice-config", callHandler.IceConfig)
		})
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	stdLog.Printf("Call service starting on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		logger.Fatal("server failed", zap.Error(err))
	}
}

// callHub публикует события звонков в кластерный канал чата.
type callHub struct {
	backend ws.ClusterBackend
}

func newCallHub(rdb *redis.Client, channel string) *callHub {
	return &callHub{backend: ws.NewRedisClusterBackend(rdb, channel)}
}

func (h *callHub) SendToUser(userID string, data []byte) {
	_ = ws.PublishCluster(h.backend, []string{userID}, data)
}

func buildIceServers(cfg *Config) []service.IceServer {
	servers := []service.IceServer{{
		URLs: []string{"stun:stun.l.google.com:19302", "stun:stun1.l.google.com:19302"},
	}}
	if cfg.TurnURL != "" {
		servers = append(servers, service.IceServer{
			URLs:       []string{cfg.TurnURL},
			Username:   cfg.TurnUsername,
			Credential: cfg.TurnPassword,
		})
	}
	return servers
}

func envStr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
