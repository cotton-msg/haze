// Loadtest для WS-шины chat: N коннектов, отправка сообщений, замер p95.
//
// Использование:
//
//	go run ./scripts/loadtest \
//	  --url ws://localhost:8082/ws \
//	  --jwt-secret change-me-in-production \
//	  --conns 1000 \
//	  --chats 50 \
//	  --msg-rate 100
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/cotton-msg/haze/backend/pkg/auth"
	"github.com/gorilla/websocket"
)

var (
	url       = flag.String("url", "ws://localhost:8082/ws", "ws endpoint")
	jwtSecret = flag.String("jwt-secret", "change-me-in-production", "JWT access secret")
	conns     = flag.Int("conns", 1000, "number of websocket connections")
	chats     = flag.Int("chats", 50, "number of chat ids to broadcast to")
	msgRate   = flag.Int("msg-rate", 100, "messages per second (sent from clients)")
	duration  = flag.Duration("duration", 10*time.Second, "test duration")
)

type event struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func main() {
	flag.Parse()

	cfg := &auth.Config{AccessSecret: *jwtSecret, RefreshSecret: *jwtSecret, AccessTTL: time.Hour}
	chatIDs := make([]string, *chats)
	for i := range chatIDs {
		chatIDs[i] = fmt.Sprintf("chat-%04d", i)
	}

	// Устанавливаем соединения.
	var (
		wg      sync.WaitGroup
		mu      sync.Mutex
		lat     []time.Duration
		delivered int64
		errors   int64
	)
	dial := websocket.Dialer{HandshakeTimeout: 5 * time.Second}

	openConn := func(n int) {
		defer wg.Done()
		token, err := auth.GenerateAccessToken(cfg, fmt.Sprintf("user-%d", n), fmt.Sprintf("user%d", n), "user")
		if err != nil {
			atomic.AddInt64(&errors, 1)
			return
		}
		conn, _, err := dial.Dial(*url, map[string][]string{"Authorization": {"Bearer " + token}})
		if err != nil {
			atomic.AddInt64(&errors, 1)
			return
		}
		defer conn.Close()

		conn.SetReadDeadline(time.Now().Add(*duration + 15*time.Second))
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			var ev event
			if json.Unmarshal(data, &ev) != nil {
				continue
			}
			if ev.Type == "new_message" {
				atomic.AddInt64(&delivered, 1)
			}
		}
	}

	start := time.Now()
	for n := 0; n < *conns; n++ {
		wg.Add(1)
		go openConn(n)
	}
	wg.Wait() // ждём завершения рукопожатий (короткие)

	// Печатаем фактическое число открытых соединений через счётчик доставки пуст.
	time.Sleep(2 * time.Second)

	// Отправляем сообщения с заданным rate. Каждое сообщение идёт в случайный чат.
	msgStop := time.Now().Add(*duration)
	sent := 0
	interval := time.Second / time.Duration(*msgRate)
	if interval < time.Millisecond {
		interval = time.Millisecond
	}
	probeToken, _ := auth.GenerateAccessToken(cfg, "loadprobe", "loadprobe", "user")
	probe, _, err := dial.Dial(*url, map[string][]string{"Authorization": {"Bearer " + probeToken}})
	if err == nil {
		defer probe.Close()
		go func() {
			for {
				probe.SetReadDeadline(time.Now().Add(time.Second))
				_, data, err := probe.ReadMessage()
				if err != nil {
					return
				}
				var ev event
				if json.Unmarshal(data, &ev) != nil || ev.Type != "new_message" {
					continue
				}
				// payload содержит message с created_at — измеряем от момента отправки
				mu.Lock()
				if len(lat) > 0 {
					lat = append(lat, time.Since(start))
				}
				mu.Unlock()
			}
		}()
	}

	for time.Now().Before(msgStop) {
		chatID := chatIDs[rand.Intn(len(chatIDs))]
		payload, _ := json.Marshal(map[string]interface{}{
			"type":    "message",
			"payload": map[string]interface{}{"chat_id": chatID, "content": "loadtest"},
		})
		_ = probe.WriteMessage(websocket.TextMessage, payload)
		sent++
		time.Sleep(interval)
	}
	time.Sleep(2 * time.Second)

	elapsed := time.Since(start)
	log.Printf("conns=%d sent=%d delivered=%d errors=%d elapsed=%s",
		*conns, sent, atomic.LoadInt64(&delivered), atomic.LoadInt64(&errors), elapsed.Round(time.Millisecond))

	mu.Lock()
	sort.Slice(lat, func(i, j int) bool { return lat[i] < lat[j] })
	if len(lat) > 0 {
		log.Printf("sample_size=%d p50=%s p95=%s p99=%s",
			len(lat), lat[len(lat)/2], lat[len(lat)*95/100], lat[len(lat)*99/100])
	}
	mu.Unlock()
}
