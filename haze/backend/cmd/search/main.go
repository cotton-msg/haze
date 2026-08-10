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
	"github.com/cotton-msg/haze/backend/internal/service"
	"github.com/cotton-msg/haze/backend/pkg/auth"
	"github.com/cotton-msg/haze/backend/pkg/config"
	"github.com/cotton-msg/haze/backend/pkg/metrics"
	"github.com/cotton-msg/haze/backend/pkg/middleware"
	"github.com/go-chi/chi/v5"

	_ "github.com/lib/pq"
)

type Config struct {
	Port          string
	DatabaseURL   string
	JWTSecret     string
	MeiliHost     string
	MeiliAPIKey   string
	InternalToken string
}

func loadConfig() *Config {
	cfg, _ := config.Load("configs/search.yaml")
	return &Config{
		Port:          cfg.EnvStr("SEARCH_PORT", "server.port", "8087"),
		DatabaseURL:   cfg.EnvStr("DATABASE_URL", "database.url", "postgres://haze:haze@localhost:5432/haze?sslmode=disable"),
		JWTSecret:     cfg.EnvStr("JWT_SECRET", "auth.jwt_secret", "change-me-in-production"),
		MeiliHost:     cfg.EnvStr("MEILI_URL", "meilisearch.url", "http://localhost:7700"),
		MeiliAPIKey:   cfg.EnvStr("MEILI_MASTER_KEY", "meilisearch.api_key", ""),
		InternalToken: cfg.EnvStr("SEARCH_INTERNAL_TOKEN", "server.internal_token", ""),
	}
}

func main() {
	cfg := loadConfig()
	logger := log.New(os.Stdout, "[search] ", log.LstdFlags)

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Fatalf("db: %v", err)
	}
	defer db.Close()

	searchSvc, err := service.NewSearchService(cfg.MeiliHost, cfg.MeiliAPIKey)
	if err != nil {
		logger.Fatalf("meilisearch: %v", err)
	}
	if err := searchSvc.EnsureIndexes(); err != nil {
		logger.Printf("warning: failed to ensure indexes: %v", err)
	}

	userRepo := repository.NewUserRepository(db)
	msgRepo := repository.NewMessageRepository(db)
	searchHandler := handler.NewSearchHandler(searchSvc, userRepo, msgRepo)

	// Первичная синхронизация в фоне
	go func() {
		time.Sleep(2 * time.Second)
		if err := searchHandler.SyncAll(); err != nil {
			logger.Printf("initial sync failed: %v", err)
		} else {
			logger.Printf("initial index sync complete")
		}
	}()

	jwtCfg := &auth.Config{AccessSecret: cfg.JWTSecret, RefreshSecret: cfg.JWTSecret}
	jwtMw := auth.JWTMiddleware(jwtCfg)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"status":"ok","service":"search"}`))
	})
	r.Handle("/metrics", metrics.Handler())

	r.Route("/search", func(r chi.Router) {
		// Публичный поиск — только для авторизованных пользователей.
		r.Group(func(r chi.Router) {
			r.Use(jwtMw)
			r.Get("/messages", searchHandler.SearchMessages)
			r.Get("/users", searchHandler.SearchUsers)
		})

		// Внутренние эндпоинты индексации (service-to-service).
		// Защищены отдельным внутренним токеном, а не JWT-пользователя.
		r.Group(func(r chi.Router) {
			r.Use(internalTokenMiddleware(cfg.InternalToken))
			r.Post("/index/user", searchHandler.IndexUser)
			r.Post("/index/message", searchHandler.IndexMessage)
			r.Post("/sync", searchHandler.Sync)
		})
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	logger.Printf("Search service on %s (meili=%s)", addr, cfg.MeiliHost)
	http.ListenAndServe(addr, r)
}

func envStr(k, d string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return d
}

// internalTokenMiddleware защищает внутренние эндпоинты сервиса.
// Если токен не задан (dev-режим) — пропускает всё.
func internalTokenMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if token != "" && r.Header.Get("X-Internal-Token") != token {
				w.WriteHeader(http.StatusUnauthorized)
				w.Write([]byte(`{"error":true,"message":"unauthorized"}`))
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
