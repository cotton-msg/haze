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
	"github.com/cotton-msg/haze/backend/pkg/crypto"
	"github.com/cotton-msg/haze/backend/pkg/metrics"
	"github.com/cotton-msg/haze/backend/pkg/middleware"
	"github.com/cotton-msg/haze/backend/pkg/queue"
	"github.com/go-chi/chi/v5"
	"github.com/redis/go-redis/v9"

	_ "github.com/lib/pq"
)

type Config struct {
	Port            string
	DatabaseURL     string
	JWTSecret       string
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	VAPIDSubject    string

	RedisAddr     string
	RedisPassword string
	RedisDB       int
	QueueStream   string

	EncryptionKey string
}

func loadConfig() *Config {
	cfg, _ := config.Load("configs/notification.yaml")
	return &Config{
		Port:            cfg.EnvStr("NOTIF_PORT", "server.port", "8086"),
		DatabaseURL:     cfg.EnvStr("DATABASE_URL", "database.url", "postgres://haze:haze@localhost:5432/haze?sslmode=disable"),
		JWTSecret:       cfg.EnvStr("JWT_SECRET", "auth.jwt_secret", "change-me-in-production"),
		VAPIDPublicKey:  cfg.EnvStr("VAPID_PUBLIC_KEY", "web_push.vapid_public_key", ""),
		VAPIDPrivateKey: cfg.EnvStr("VAPID_PRIVATE_KEY", "web_push.vapid_private_key", ""),
		VAPIDSubject:    cfg.EnvStr("VAPID_SUBJECT", "web_push.vapid_subject", "mailto:admin@haze.local"),

		RedisAddr:     cfg.EnvStr("REDIS_ADDR", "redis.address", "localhost:6379"),
		RedisPassword: cfg.EnvStr("REDIS_PASSWORD", "redis.password", ""),
		RedisDB:       cfg.EnvInt("REDIS_DB", "redis.db", 2),
		QueueStream:   cfg.EnvStr("QUEUE_STREAM", "queue.stream", "haze:notif"),

		EncryptionKey: cfg.EnvStr("ENCRYPTION_KEY", "database.encryption_key", ""),
	}
}

func main() {
	cfg := loadConfig()
	logger := log.New(os.Stdout, "[notif] ", log.LstdFlags)

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Fatalf("db: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)

	encBox, err := crypto.New(cfg.EncryptionKey)
	if err != nil {
		logger.Printf("at-rest encryption disabled: %v", err)
	}
	pushRepo := repository.NewPushRepository(db, encBox)
	userRepo := repository.NewUserRepository(db)
	pushSvc := service.NewPushService(service.PushConfig{
		VAPIDPublicKey:  cfg.VAPIDPublicKey,
		VAPIDPrivateKey: cfg.VAPIDPrivateKey,
		VAPIDSubject:    cfg.VAPIDSubject,
	}, pushRepo)
	notifHandler := handler.NewNotificationHandler(pushRepo, pushSvc, userRepo)

	// Потребитель очереди Redis Streams (chat → notification).
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	consumer := queue.NewConsumer(rdb, cfg.QueueStream, "notif-workers", func(msg queue.Message) error {
		if msg.Type != "new_message" {
			return nil
		}
		var req handler.SendRequest
		if err := json.Unmarshal(msg.Body, &req); err != nil {
			return err
		}
		notifHandler.ProcessSend(req)
		return nil
	})
	go func() {
		if err := rdb.Ping(context.Background()).Err(); err == nil {
			logger.Printf("queue consumer: redis stream (%s)", cfg.QueueStream)
			_ = consumer.Run(context.Background(), time.Second)
		} else {
			logger.Printf("redis unavailable (%v) — queue consumer disabled", err)
		}
	}()

	jwtCfg := &auth.Config{AccessSecret: cfg.JWTSecret, RefreshSecret: cfg.JWTSecret}
	jwtMw := auth.JWTMiddleware(jwtCfg)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok","service":"notification"}`))
	})
	r.Handle("/metrics", metrics.Handler())

	r.Route("/notifications", func(r chi.Router) {
		r.Get("/vapid", notifHandler.Vapid)
		r.Group(func(r chi.Router) {
			r.Use(jwtMw)
			r.Post("/register", notifHandler.Register)
			r.Delete("/register", notifHandler.Unregister)
			r.Put("/settings", notifHandler.SaveSettings)
		})
		r.Post("/send", notifHandler.Send)
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	logger.Printf("Notification service on %s", addr)
	http.ListenAndServe(addr, r)
}

func envStr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
