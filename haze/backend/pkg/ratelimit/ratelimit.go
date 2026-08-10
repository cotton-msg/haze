package ratelimit

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// Limiter — интерфейс rate limiter'а (fixed window).
type Limiter interface {
	Allow(key string) bool
}

// MemoryLimiter — фиксированное окно в памяти (fallback без Redis).
type MemoryLimiter struct {
	mu      sync.Mutex
	clients map[string]*entry
	limit   int
	window  time.Duration
}

type entry struct {
	count   int
	resetAt time.Time
}

func NewMemoryLimiter(limit int, window time.Duration) *MemoryLimiter {
	return &MemoryLimiter{
		clients: make(map[string]*entry),
		limit:   limit,
		window:  window,
	}
}

func (rl *MemoryLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	e, exists := rl.clients[key]
	if !exists || now.After(e.resetAt) {
		rl.clients[key] = &entry{count: 1, resetAt: now.Add(rl.window)}
		return true
	}
	if e.count >= rl.limit {
		return false
	}
	e.count++
	return true
}

// RedisLimiter — фиксированное окно на Redis (работает в кластере).
type RedisLimiter struct {
	client redis.Cmdable
	limit  int
	window time.Duration
	prefix string
}

func NewRedisLimiter(client redis.Cmdable, limit int, window time.Duration, prefix string) *RedisLimiter {
	return &RedisLimiter{client: client, limit: limit, window: window, prefix: prefix}
}

// incrIfExistsScript инкрементирует счётчик и устанавливает TTL только при
// первом создании ключа. Без продления TTL на каждом запросе блокировка
// истекает вместе с окном, а не «навсегда».
var incrIfExistsScript = redis.NewScript(`
local key = KEYS[1]
local limit = tonumber(ARGV[1])
local ttl = tonumber(ARGV[2])
local n = redis.call('INCR', key)
if n == 1 then
    redis.call('PEXPIRE', key, ttl)
end
if n > limit then
    return 0
end
return 1
`)

func (rl *RedisLimiter) Allow(key string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	rkey := fmt.Sprintf("%s:%s", rl.prefix, key)
	n, err := incrIfExistsScript.Run(ctx, rl.client, []string{rkey}, rl.limit, rl.window.Milliseconds()).Int()
	if err != nil {
		return false
	}
	return n == 1
}
