package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/cotton-msg/haze/backend/pkg/config"
	"github.com/cotton-msg/haze/backend/pkg/metrics"
	"github.com/cotton-msg/haze/backend/pkg/ratelimit"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	Port        int
	LogLevel    string
	Services    map[string]string
	RateLimit   int
	RateWindow  time.Duration
	CORSOrigins []string

	RedisAddr      string
	RedisPassword  string
	RedisDB        int
	RedisKeyPrefix string
}

func loadConfig() *Config {
	cfg, err := config.Load(envStr("CONFIG_FILE", "configs/gateway.yaml"))
	if err != nil {
		log.Printf("config warning: %v", err)
	}

	services := map[string]string{
		"/api/auth":         cfg.EnvStr("SERVICE_AUTH", "services.auth", "http://localhost:8081"),
		"/api/chat":         cfg.EnvStr("SERVICE_CHAT", "services.chat", "http://localhost:8082"),
		"/api/media":        cfg.EnvStr("SERVICE_MEDIA", "services.media", "http://localhost:8083"),
		"/api/call":         cfg.EnvStr("SERVICE_CALL", "services.call", "http://localhost:8084"),
		"/api/bot":          cfg.EnvStr("SERVICE_BOT", "services.bot", "http://localhost:8085"),
		"/api/notification": cfg.EnvStr("SERVICE_NOTIFICATION", "services.notification", "http://localhost:8086"),
		"/api/search":       cfg.EnvStr("SERVICE_SEARCH", "services.search", "http://localhost:8087"),
		"/api/premium":      cfg.EnvStr("SERVICE_PREMIUM", "services.premium", "http://localhost:8088"),
		"/api/admin":        cfg.EnvStr("SERVICE_AUTH", "services.auth", "http://localhost:8081"),
	}

	return &Config{
		Port:           cfg.EnvInt("GATEWAY_PORT", "server.port", 8080),
		LogLevel:       cfg.EnvStr("GATEWAY_LOG_LEVEL", "server.log_level", "info"),
		RateLimit:      cfg.EnvInt("GATEWAY_RATE_LIMIT", "rate_limit.default", 100),
		RateWindow:     cfg.EnvDuration("GATEWAY_RATE_WINDOW", "rate_limit.window", time.Minute),
		CORSOrigins:    cfg.EnvStrSlice("GATEWAY_CORS_ORIGIN", "cors.allowed_origins", []string{"http://localhost:3000"}),
		RedisAddr:      cfg.EnvStr("REDIS_ADDR", "redis.address", "localhost:6379"),
		RedisPassword:  cfg.EnvStr("REDIS_PASSWORD", "redis.password", ""),
		RedisDB:        cfg.EnvInt("REDIS_DB", "redis.db", 0),
		RedisKeyPrefix: "haze:rl",
		Services:       services,
	}
}

func corsMiddleware(next http.Handler, origins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		allowed := ""
		for _, o := range origins {
			if o == "*" || o == origin {
				allowed = origin
				if o == "*" {
					allowed = "*"
				}
				break
			}
		}

		if allowed != "" {
			w.Header().Set("Access-Control-Allow-Origin", allowed)
			if allowed != "*" {
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
		}
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		w.Header().Set("Access-Control-Max-Age", "86400")

		// Заголовки безопасности.
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-XSS-Protection", "0")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' https://*.haze.app wss://*.haze.app; img-src 'self' data: blob: https://*.haze.app; media-src 'self' data: blob:")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	logger := log.New(os.Stdout, "[gateway] ", log.LstdFlags)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rid := r.Header.Get("X-Request-ID")
		if rid == "" {
			rid = fmt.Sprintf("gw-%d", time.Now().UnixNano())
		}
		w.Header().Set("X-Request-ID", rid)

		lrw := &logResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(lrw, r)

		metrics.HTTPRequests.WithLabelValues("gateway", r.Method, r.URL.Path, metrics.StatusFromCode(lrw.statusCode)).Inc()
		metrics.HTTPDuration.WithLabelValues("gateway", r.Method, r.URL.Path).Observe(time.Since(start).Seconds())
		logger.Printf("%s %s %d %s [%s]", r.Method, r.URL.Path, lrw.statusCode, time.Since(start), rid)
	})
}

type logResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *logResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func proxyHandler(target string) http.Handler {
	u, err := url.Parse(target)
	if err != nil {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			jsonError(w, http.StatusInternalServerError, fmt.Sprintf("bad target: %s", target))
		})
	}
	proxy := httputil.NewSingleHostReverseProxy(u)
	return proxy
}

func jsonError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{"error": true, "message": msg})
}

func jsonOK(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"error": false, "data": data})
}

func main() {
	cfg := loadConfig()
	logger := log.New(os.Stdout, "[gateway] ", log.LstdFlags)

	// Rate limiter: Redis в приоритете, fallback — память.
	var rl ratelimit.Limiter
	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
		DB:       cfg.RedisDB,
	})
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Printf("redis unavailable (%v) — using in-memory rate limiter", err)
		rl = ratelimit.NewMemoryLimiter(cfg.RateLimit, cfg.RateWindow)
	} else {
		rl = ratelimit.NewRedisLimiter(rdb, cfg.RateLimit, cfg.RateWindow, cfg.RedisKeyPrefix)
		logger.Printf("rate limiter: redis (%s)", cfg.RedisAddr)
	}
	cancel()

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, map[string]string{"status": "ok", "service": "gateway"})
	})
	mux.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		jsonOK(w, map[string]string{"status": "ready"})
	})
	mux.Handle("/metrics", metrics.Handler())

	for prefix, target := range cfg.Services {
		handler := proxyHandler(target)
		p := prefix
		t := target
		mux.Handle(prefix+"/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.URL.Path = strings.TrimPrefix(r.URL.Path, p)
			if !rl.Allow(clientKey(r)) {
				w.Header().Set("Retry-After", "60")
				jsonError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}
			handler.ServeHTTP(w, r)
			logger.Printf("proxy %s -> %s%s", p, t, r.URL.Path)
		}))
	}

	var h http.Handler = mux
	h = loggingMiddleware(h)
	h = corsMiddleware(h, cfg.CORSOrigins)

	logger.Printf("Gateway starting on :%d with %d routes", cfg.Port, len(cfg.Services))
	if err := http.ListenAndServe(fmt.Sprintf(":%d", cfg.Port), h); err != nil {
		logger.Fatal(err)
	}
}

func clientKey(r *http.Request) string {
	if uid := r.Header.Get("X-User-ID"); uid != "" {
		return uid
	}
	ip := r.RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx > 0 {
		ip = ip[:idx]
	}
	return ip
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
