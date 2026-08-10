package ratelimit

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestRedisLimiter_BlockReleasedAfterWindow(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer s.Close()

	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer client.Close()

	window := 50 * time.Millisecond
	rl := NewRedisLimiter(client, 2, window, "rl:test")

	if !rl.Allow("ip:1") {
		t.Fatal("first request should be allowed")
	}
	if !rl.Allow("ip:1") {
		t.Fatal("second request should be allowed")
	}
	if rl.Allow("ip:1") {
		t.Fatal("third request should be blocked (limit 2)")
	}

	// TTL не должен продлеваться на каждом запросе: дожидаемся конца окна.
	s.FastForward(window + 10*time.Millisecond)

	if !rl.Allow("ip:1") {
		t.Fatal("after window expiry the key should reset and allow again")
	}
}

func TestRedisLimiter_IndependentKeys(t *testing.T) {
	s, err := miniredis.Run()
	if err != nil {
		t.Fatalf("miniredis: %v", err)
	}
	defer s.Close()

	client := redis.NewClient(&redis.Options{Addr: s.Addr()})
	defer client.Close()

	rl := NewRedisLimiter(client, 1, time.Minute, "rl:test")

	if !rl.Allow("ip:a") {
		t.Fatal("first key should be allowed")
	}
	if rl.Allow("ip:a") {
		t.Fatal("same key over limit should be blocked")
	}
	if !rl.Allow("ip:b") {
		t.Fatal("different key must not share the counter")
	}
}
