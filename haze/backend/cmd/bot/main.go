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
	Port        string
	DatabaseURL string
	JWTSecret   string
}

func loadConfig() *Config {
	raw, _ := config.Load("configs/bot.yaml")
	return &Config{
		Port:        raw.EnvStr("BOT_PORT", "server.port", "8085"),
		DatabaseURL: raw.EnvStr("DATABASE_URL", "database.url", "postgres://haze:haze@localhost:5432/haze?sslmode=disable"),
		JWTSecret:   raw.EnvStr("JWT_SECRET", "auth.jwt_secret", "change-me-in-production"),
	}
}

func main() {
	cfg := loadConfig()
	logger := log.New(os.Stdout, "[bot] ", log.LstdFlags)

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Fatalf("db: %v", err)
	}
	defer db.Close()
	db.SetMaxOpenConns(10)
	db.SetConnMaxLifetime(5 * time.Minute)

	botRepo := repository.NewBotRepository(db)
	cmdRepo := repository.NewBotCommandRepository(db)
	userRepo := repository.NewUserRepository(db)
	botHandler := handler.NewBotHandler(botRepo, cmdRepo, userRepo)

	jwtCfg := &auth.Config{
		AccessSecret: cfg.JWTSecret, RefreshSecret: cfg.JWTSecret,
		AccessTTL: 15 * time.Minute, RefreshTTL: 720 * time.Hour,
	}

	r := chi.NewRouter()
	r.Use(middleware.RequestID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok","service":"bot"}`))
	})
	r.Handle("/metrics", metrics.Handler())

	r.Route("/bot", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(auth.JWTMiddleware(jwtCfg))
			r.Post("/create", botHandler.Create)
			r.Get("/list", botHandler.List)
		})
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	logger.Printf("Bot service on %s", addr)
	http.ListenAndServe(addr, r)
}

func envStr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}
