package ws

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/cotton-msg/haze/backend/pkg/auth"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

func newTestClient(t *testing.T, hub *Hub, userID string) *Client {
	t.Helper()
	logger, _ := zap.NewDevelopment()
	client := NewClient(userID, nil, hub, logger)
	hub.Register(client)
	return client
}

func TestHubRegisterUnregister(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	hub := NewHub(logger)

	c1 := newTestClient(t, hub, "alice")
	c2 := newTestClient(t, hub, "alice")
	newTestClient(t, hub, "bob")

	if hub.GetUserCount("alice") != 2 {
		t.Errorf("expected 2 alice clients, got %d", hub.GetUserCount("alice"))
	}
	if hub.GetUserCount("bob") != 1 {
		t.Errorf("expected 1 bob client, got %d", hub.GetUserCount("bob"))
	}
	if !hub.IsOnline("alice") || !hub.IsOnline("bob") {
		t.Error("both users should be online")
	}

	hub.Unregister(c1)
	if hub.GetUserCount("alice") != 1 {
		t.Errorf("expected 1 alice client after unregister, got %d", hub.GetUserCount("alice"))
	}

	hub.Unregister(c2)
	if hub.IsOnline("alice") {
		t.Error("alice should be offline after all clients unregister")
	}
	if len(hub.clients) != 1 {
		t.Errorf("expected only bob left in map, got %d keys", len(hub.clients))
	}
}

func TestSendToUserDeliversToAllClients(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	hub := NewHub(logger)

	c1 := newTestClient(t, hub, "alice")
	c2 := newTestClient(t, hub, "alice")

	payload := []byte(`{"type":"status","user_id":"alice"}`)
	hub.SendToUser("alice", payload)

	for i, c := range []*Client{c1, c2} {
		select {
		case msg := <-c.Send:
			if !bytes.Equal(msg, payload) {
				t.Errorf("client %d got %s, want %s", i, msg, payload)
			}
		case <-time.After(time.Second):
			t.Fatalf("client %d did not receive message", i)
		}
	}
}

func TestSendToUserUnknownUserIsNoop(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	hub := NewHub(logger)
	hub.SendToUser("ghost", []byte(`{"x":1}`))
}

func TestBroadcastToChatWithoutBackend(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	hub := NewHub(logger)

	a1 := newTestClient(t, hub, "alice")
	b1 := newTestClient(t, hub, "bob")
	_ = newTestClient(t, hub, "eve") // не в чате

	payload := []byte(`{"type":"message"}`)
	hub.BroadcastToChat([]string{"alice", "bob"}, payload)

	for _, c := range []*Client{a1, b1} {
		select {
		case msg := <-c.Send:
			if !bytes.Equal(msg, payload) {
				t.Errorf("got %s, want %s", msg, payload)
			}
		case <-time.After(time.Second):
			t.Fatal("chat member did not receive broadcast")
		}
	}
}

type mockClusterBackend struct {
	mu        sync.Mutex
	published [][]byte
	handlers  []func([]byte)
}

func (m *mockClusterBackend) Publish(payload []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.published = append(m.published, payload)
	return nil
}

func (m *mockClusterBackend) Subscribe(_ context.Context, handler func([]byte)) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.handlers = append(m.handlers, handler)
	return nil
}

func (m *mockClusterBackend) publishCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.published)
}

func TestBroadcastToChatUsesClusterBackend(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	hub := NewHub(logger)
	backend := &mockClusterBackend{}
	hub.WithClusterBackend(backend)

	hub.BroadcastToChat([]string{"alice"}, []byte(`{"type":"message"}`))

	if backend.publishCount() != 1 {
		t.Fatalf("expected 1 cluster publish, got %d", backend.publishCount())
	}
}

func TestHandleMessageTypingBroadcastsToSender(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	hub := NewHub(logger)
	sender := newTestClient(t, hub, "alice")

	data := []byte(`{"type":"typing","chat_id":"c1"}`)
	hub.HandleMessage(sender, MsgTyping, data)

	select {
	case msg := <-sender.Send:
		if !bytes.Equal(msg, data) {
			t.Errorf("got %s, want %s", msg, data)
		}
	case <-time.After(time.Second):
		t.Fatal("sender did not receive typing broadcast")
	}
}

func makeTestToken(t *testing.T, secret string, userID string) string {
	t.Helper()
	cfg := &auth.Config{
		AccessSecret: secret,
		AccessTTL:    time.Hour,
	}
	tok, err := auth.GenerateAccessToken(cfg, userID, "tester", "user")
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	return tok
}

func TestServeWSRequiresToken(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	hub := NewHub(logger)
	handler := NewWSHandler(hub, logger)

	srv := httptest.NewServer(http.HandlerFunc(handler.ServeWS))
	defer srv.Close()

	resp, err := http.Get(srv.URL)
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 without token, got %d", resp.StatusCode)
	}
}

func TestServeWSRejectsBadToken(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	hub := NewHub(logger)
	handler := NewWSHandler(hub, logger)

	cfg := &auth.Config{AccessSecret: "secret"}
	srv := httptest.NewServer(auth.WSJWTMiddleware(cfg)(http.HandlerFunc(handler.ServeWS)))
	defer srv.Close()

	resp, err := http.Get(srv.URL + "?token=invalid")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 for bad token, got %d", resp.StatusCode)
	}
}

func TestServeWSHandshakeAndBroadcast(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	hub := NewHub(logger)
	handler := NewWSHandler(hub, logger)

	cfg := &auth.Config{AccessSecret: "secret"}
	token := makeTestToken(t, cfg.AccessSecret, "alice")

	srv := httptest.NewServer(auth.WSJWTMiddleware(cfg)(http.HandlerFunc(handler.ServeWS)))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):] + "?token=" + token
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	deadline := time.Now().Add(2 * time.Second)
	for hub.GetUserCount("alice") != 1 {
		if time.Now().After(deadline) {
			t.Fatal("client did not register in hub")
		}
		time.Sleep(10 * time.Millisecond)
	}

	payload := []byte(`{"type":"status","online":true}`)
	hub.SendToUser("alice", payload)

	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read message: %v", err)
	}
	if !bytes.Equal(msg, payload) {
		t.Errorf("payload mismatch: got %s want %s", msg, payload)
	}
}

func TestServeWSSendTypingThroughSocket(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	hub := NewHub(logger)
	handler := NewWSHandler(hub, logger)

	cfg := &auth.Config{AccessSecret: "secret"}
	token := makeTestToken(t, cfg.AccessSecret, "alice")

	srv := httptest.NewServer(auth.WSJWTMiddleware(cfg)(http.HandlerFunc(handler.ServeWS)))
	defer srv.Close()

	wsURL := "ws" + srv.URL[len("http"):] + "?token=" + token
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	msg := Message{Type: MsgTyping, Payload: json.RawMessage(`{"chat_id":"c1"}`)}
	if err := conn.WriteJSON(msg); err != nil {
		t.Fatalf("write: %v", err)
	}

	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read echo: %v", err)
	}
	var echo Message
	if err := json.Unmarshal(data, &echo); err != nil {
		t.Fatalf("unmarshal echo: %v, raw=%s", err, data)
	}
	if echo.Type != MsgTyping {
		t.Errorf("expected typing echo, got %s", echo.Type)
	}
}
