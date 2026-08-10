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
	Port        string
	DatabaseURL string
	StorageType string
	MediaDir    string
	MediaURL    string
	MaxSize     int64
	MaxSizeP    int64
	JWTSecret   string
	LogLevel    string

	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioUseSSL    bool
	MinioPublicURL string
}

func loadConfig() *Config {
	cfg, _ := config.Load("configs/media.yaml")
	return &Config{
		Port:        cfg.EnvStr("MEDIA_PORT", "server.port", "8083"),
		DatabaseURL: cfg.EnvStr("DATABASE_URL", "database.url", "postgres://haze:haze@localhost:5432/haze?sslmode=disable"),
		StorageType: cfg.EnvStr("MEDIA_STORAGE", "storage.type", "local"),
		MediaDir:    cfg.EnvStr("MEDIA_DIR", "storage.local_dir", "./media"),
		MediaURL:    cfg.EnvStr("MEDIA_URL", "storage.local_url", "http://localhost:8083/media"),
		MaxSize:     int64(cfg.EnvInt("MEDIA_MAX_SIZE", "limits.max_file_size", 50<<20)),
		MaxSizeP:    int64(cfg.EnvInt("MEDIA_MAX_SIZE_PREMIUM", "limits.max_file_size_premium", 500<<20)),
		JWTSecret:   cfg.EnvStr("JWT_SECRET", "auth.jwt_secret", "change-me-in-production"),
		LogLevel:    cfg.EnvStr("MEDIA_LOG_LEVEL", "server.log_level", "info"),

		MinioEndpoint:  cfg.EnvStr("MINIO_ENDPOINT", "minio.endpoint", "localhost:9000"),
		MinioAccessKey: cfg.EnvStr("MINIO_ACCESS_KEY", "minio.access_key", "minioadmin"),
		MinioSecretKey: cfg.EnvStr("MINIO_SECRET_KEY", "minio.secret_key", "minioadmin"),
		MinioBucket:    cfg.EnvStr("MINIO_BUCKET", "minio.bucket", "haze-media"),
		MinioUseSSL:    cfg.EnvBool("MINIO_USE_SSL", "minio.use_ssl", false),
		MinioPublicURL: cfg.EnvStr("MINIO_PUBLIC_URL", "minio.public_url", ""),
	}
}

func main() {
	cfg := loadConfig()
	logger := log.New(os.Stdout, "[media] ", log.LstdFlags)

	db, err := sql.Open("postgres", cfg.DatabaseURL)
	if err != nil {
		logger.Fatalf("failed to connect to database: %v", err)
	}
	defer db.Close()

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		logger.Printf("warning: database not reachable: %v", err)
	}

	fileRepo := repository.NewFileRepository(db)

	var storage service.StorageBackend
	if cfg.StorageType == "minio" {
		minioStorage, err := service.NewMinioStorage(service.MinioConfig{
			Endpoint:  cfg.MinioEndpoint,
			AccessKey: cfg.MinioAccessKey,
			SecretKey: cfg.MinioSecretKey,
			Bucket:    cfg.MinioBucket,
			UseSSL:    cfg.MinioUseSSL,
			PublicURL: cfg.MinioPublicURL,
		})
		if err != nil {
			logger.Fatalf("minio storage: %v", err)
		}
		storage = minioStorage
		logger.Printf("storage backend: minio (%s/%s)", cfg.MinioEndpoint, cfg.MinioBucket)
	} else {
		storage = service.NewLocalStorage(cfg.MediaDir, cfg.MediaURL)
		logger.Printf("storage backend: local (%s)", cfg.MediaDir)
	}

	mediaSvc := service.NewMediaService(fileRepo, storage, cfg.MaxSize, cfg.MaxSizeP)
	mediaHandler := handler.NewMediaHandler(mediaSvc)

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
		w.Write([]byte(`{"status":"ok","service":"media"}`))
	})
	r.Handle("/metrics", metrics.Handler())

	r.Route("/media", func(r chi.Router) {
		r.Group(func(r chi.Router) {
			r.Use(auth.JWTMiddleware(jwtCfg))
			r.Post("/upload", mediaHandler.Upload)
			r.Get("/{id}", mediaHandler.GetFile)
			r.Get("/{id}/presign", mediaHandler.Presign)
		})
	})

	r.Get("/media/files/*", mediaHandler.ServeFile)

	addr := fmt.Sprintf(":%s", cfg.Port)
	logger.Printf("Media service starting on %s", addr)
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
