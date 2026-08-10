package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cotton-msg/haze/backend/internal/handler"
	"github.com/cotton-msg/haze/backend/internal/repository"
	"github.com/cotton-msg/haze/backend/pkg/auth"
	"github.com/cotton-msg/haze/backend/pkg/config"
	"github.com/cotton-msg/haze/backend/pkg/metrics"
	"github.com/cotton-msg/haze/backend/pkg/middleware"
	"github.com/go-chi/chi/v5"

	_ "github.com/lib/pq"
)

type Config struct {
	Port            string
	DatabaseURL     string
	JWTSecret       string
	StripeSecretKey string
	StripeWebhook   string
	StripeSuccess   string
	StripeCancel    string
}

func loadConfig() *Config {
	cfg, _ := config.Load("configs/premium.yaml")
	return &Config{
		Port:            cfg.EnvStr("PREMIUM_PORT", "server.port", "8088"),
		DatabaseURL:     cfg.EnvStr("DATABASE_URL", "database.url", "postgres://haze:haze@localhost:5432/haze?sslmode=disable"),
		JWTSecret:       cfg.EnvStr("JWT_SECRET", "auth.jwt_secret", "change-me-in-production"),
		StripeSecretKey: cfg.EnvStr("STRIPE_SECRET_KEY", "stripe.secret_key", ""),
		StripeWebhook:   cfg.EnvStr("STRIPE_WEBHOOK_SECRET", "stripe.webhook_secret", ""),
		StripeSuccess:   cfg.EnvStr("STRIPE_SUCCESS_URL", "stripe.success_url", "https://haze.app/premium/success"),
		StripeCancel:    cfg.EnvStr("STRIPE_CANCEL_URL", "stripe.cancel_url", "https://haze.app/premium/cancel"),
	}
}

func main() {
	cfg := loadConfig()
	logger := log.New(os.Stdout, "[premium] ", log.LstdFlags)

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Fatalf("db: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	premiumRepo := repository.NewPremiumRepository(db)
	premiumHandler := handler.NewPremiumHandler(premiumRepo, &handler.StripeConfig{
		SecretKey:     cfg.StripeSecretKey,
		WebhookSecret: cfg.StripeWebhook,
		SuccessURL:    cfg.StripeSuccess,
		CancelURL:     cfg.StripeCancel,
	})

	if cfg.StripeSecretKey == "" {
		logger.Printf("STRIPE_SECRET_KEY not set — running in mock mode (no real payments)")
	}

	jwtCfg := &auth.Config{
		AccessSecret: cfg.JWTSecret, RefreshSecret: cfg.JWTSecret,
		AccessTTL: 15 * time.Minute, RefreshTTL: 720 * time.Hour,
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok","service":"premium"}`))
	})
	r.Handle("/metrics", metrics.Handler())

	r.Route("/premium", func(r chi.Router) {
		r.Get("/plans", premiumHandler.GetPlans)
		r.Post("/webhook", premiumHandler.Webhook)
		r.Group(func(r chi.Router) {
			r.Use(auth.JWTMiddleware(jwtCfg))
			r.Get("/status", premiumHandler.GetStatus)
			r.Post("/subscribe", premiumHandler.Subscribe)
			r.Post("/cancel", premiumHandler.Cancel)
		})
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	logger.Printf("Premium service on %s", addr)
	http.ListenAndServe(addr, r)
}

func envStr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
