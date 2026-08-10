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
	"github.com/redis/go-redis/v9"

	_ "github.com/lib/pq"
)

type Config struct {
	Port        string
	DatabaseURL string
	RedisAddr   string
	RedisDB     int
	LogLevel    string

	SSAClientID     string
	SSAClientSecret string
	SSAAuthorizeURL string
	SSATokenURL     string
	SSARedirectURL  string

	JWTSecret     string
	JWTRefreshTTL time.Duration
}

func loadConfig() *Config {
	raw, _ := config.Load("configs/auth.yaml")
	return &Config{
		Port:        raw.EnvStr("AUTH_PORT", "server.port", "8081"),
		DatabaseURL: raw.EnvStr("DATABASE_URL", "database.url", "postgres://haze:haze@localhost:5432/haze?sslmode=disable"),
		RedisAddr:   raw.EnvStr("REDIS_ADDR", "redis.address", "localhost:6379"),
		RedisDB:     raw.EnvInt("REDIS_DB", "redis.db", 0),
		LogLevel:    raw.EnvStr("AUTH_LOG_LEVEL", "server.log_level", "info"),

		SSAClientID:     raw.EnvStr("SSA_CLIENT_ID", "ssa.client_id", ""),
		SSAClientSecret: raw.EnvStr("SSA_CLIENT_SECRET", "ssa.client_secret", ""),
		SSAAuthorizeURL: raw.EnvStr("SSA_AUTHORIZE_URL", "ssa.authorize_url", ""),
		SSATokenURL:     raw.EnvStr("SSA_TOKEN_URL", "ssa.token_url", ""),
		SSARedirectURL:  raw.EnvStr("SSA_REDIRECT_URL", "ssa.redirect_url", "http://localhost:8080/api/auth/ssa/callback"),

		JWTSecret:     raw.EnvStr("JWT_SECRET", "jwt.access_secret", "change-me-in-production"),
		JWTRefreshTTL: raw.EnvDuration("JWT_REFRESH_TTL", "jwt.refresh_ttl", 720*time.Hour),
	}
}

func main() {
	cfg := loadConfig()
	logger := log.New(os.Stdout, "[auth] ", log.LstdFlags)

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(20)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		logger.Printf("warning: database not reachable: %v (service will start anyway)", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: cfg.RedisAddr,
		DB:   cfg.RedisDB,
	})

	userRepo := repository.NewUserRepository(db)
	sessionRepo := repository.NewSessionRepository(db)
	cacheRepo := repository.NewCacheRepository(rdb)
	badgeRepo := repository.NewBadgeRepository(db)
	botRepo := repository.NewBotRepository(db)
	statsRepo := repository.NewStatsRepository(db)

	ssaCfg := &repository.SSAConfig{
		ClientID:     cfg.SSAClientID,
		ClientSecret: cfg.SSAClientSecret,
		AuthorizeURL: cfg.SSAAuthorizeURL,
		TokenURL:     cfg.SSATokenURL,
		RedirectURL:  cfg.SSARedirectURL,
	}
	ssaClient := repository.NewSSAClient(ssaCfg)

	jwtCfg := &auth.Config{
		AccessSecret:  cfg.JWTSecret,
		RefreshSecret: cfg.JWTSecret,
		AccessTTL:     15 * time.Minute,
		RefreshTTL:    cfg.JWTRefreshTTL,
	}

	svc := service.NewAuthService(userRepo, sessionRepo, cacheRepo, ssaClient, jwtCfg)
	svc.SetIndexer(service.NewHTTPIndexer(envStr("SERVICE_SEARCH", "http://localhost:8087")).SetInternalToken(envStr("SEARCH_INTERNAL_TOKEN", "")))
	h := handler.NewAuthHandler(svc)
	adminHandler := handler.NewAdminHandler(badgeRepo, userRepo, botRepo, statsRepo)

	r := chi.NewRouter()

	r.Use(middleware.RequestID)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok","service":"auth"}`))
	})
	r.Handle("/metrics", metrics.Handler())

	r.Route("/auth", func(r chi.Router) {
		r.Get("/ssa/authorize", h.SSAAuthorize)
		r.Post("/ssa/callback", h.SSACallback)
		r.Post("/register", h.Register)
		r.Post("/login", h.Login)
		r.Post("/refresh", h.Refresh)

		r.Group(func(r chi.Router) {
			r.Use(auth.JWTMiddleware(jwtCfg))
			r.Post("/logout", h.Logout)
			r.Get("/me", h.GetMe)
			r.Put("/me", h.UpdateProfile)
			r.Get("/username/check/{username}", h.CheckUsername)
			r.Put("/username", h.UpdateUsername)
		})
	})

	r.Route("/admin", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(auth.JWTMiddleware(jwtCfg))
			r.Get("/dashboard", adminHandler.Dashboard)
			r.Get("/badges", adminHandler.GetAllBadges)
			r.Post("/badge", adminHandler.AssignBadge)
			r.Delete("/badge/{userId}/{badgeType}", adminHandler.RemoveBadge)
			r.Put("/user/{userId}/role", adminHandler.ChangeRole)
		})
	})

	addr := fmt.Sprintf(":%s", cfg.Port)
	logger.Printf("Auth service starting on %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		logger.Fatal(err)
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
