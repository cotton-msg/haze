package ws

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/cotton-msg/haze/backend/pkg/metrics"
	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type MessageType string

const (
	MsgMessage MessageType = "message"
	MsgTyping  MessageType = "typing"
	MsgStatus  MessageType = "status"
	MsgRead    MessageType = "read"
)

// ClusterBackend рассылает события между инстансами через Redis PubSub.
type ClusterBackend interface {
	Publish(payload []byte) error
	Subscribe(ctx context.Context, handler func(payload []byte)) error
}

type redisClusterBackend struct {
	client  *redis.Client
	channel string
}

func NewRedisClusterBackend(client *redis.Client, channel string) ClusterBackend {
	return &redisClusterBackend{client: client, channel: channel}
}

func (b *redisClusterBackend) Publish(payload []byte) error {
	return b.client.Publish(context.Background(), b.channel, payload).Err()
}

func (b *redisClusterBackend) Subscribe(ctx context.Context, handler func(payload []byte)) error {
	pubsub := b.client.Subscribe(ctx, b.channel)
	defer pubsub.Close()

	ch := pubsub.Channel()
	for msg := range ch {
		handler([]byte(msg.Payload))
	}
	return nil
}

// clusterEvent — формат события, рассылаемого между инстансами.
type clusterEvent struct {
	UserIDs []string        `json:"user_ids"`
	Type    MessageType     `json:"type"`
	Data    json.RawMessage `json:"data"`
}

// PresenceHook — уведомления о статусе подключения пользователя
// (реализация в сервисе, например на Redis).
type PresenceHook interface {
	Online(userID string)
	Offline(userID string)
	Heartbeat(userID string)
}

type Hub struct {
	mu       sync.RWMutex
	clients  map[string][]*Client
	logger   *zap.Logger
	backend  ClusterBackend
	service  string
	presence PresenceHook
}

func NewHub(logger *zap.Logger) *Hub {
	return &Hub{
		clients: make(map[string][]*Client),
		logger:  logger,
		service: "chat",
	}
}

// SetPresenceHook подключает обработчик статусов онлайн/оффлайн.
func (h *Hub) SetPresenceHook(hook PresenceHook) {
	h.presence = hook
}

// WithClusterBackend подключает Redis PubSub для рассылки между инстансами.
func (h *Hub) WithClusterBackend(b ClusterBackend) {
	h.backend = b
	ctx := context.Background()
	go func() {
		if err := b.Subscribe(ctx, func(payload []byte) {
			var ev clusterEvent
			if err := json.Unmarshal(payload, &ev); err != nil {
				h.logger.Warn("invalid cluster event", zap.Error(err))
				return
			}
			// Локальная доставка — без повторной публикации в кластер.
			for _, uid := range ev.UserIDs {
				h.SendToUser(uid, ev.Data)
			}
		}); err != nil {
			h.logger.Warn("cluster subscription stopped", zap.Error(err))
		}
	}()
}

func (h *Hub) Register(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.clients[client.UserID] = append(h.clients[client.UserID], client)
	metrics.WSConnections.WithLabelValues(h.service).Inc()
	if h.presence != nil {
		h.presence.Online(client.UserID)
	}
	h.logger.Info("WebSocket client connected", zap.String("user_id", client.UserID))
}

func (h *Hub) Unregister(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	clients := h.clients[client.UserID]
	for i, c := range clients {
		if c == client {
			h.clients[client.UserID] = append(clients[:i], clients[i+1:]...)
			break
		}
	}
	if len(h.clients[client.UserID]) == 0 {
		delete(h.clients, client.UserID)
	}
	metrics.WSConnections.WithLabelValues(h.service).Dec()
	if h.presence != nil {
		h.presence.Offline(client.UserID)
	}
	close(client.Send)
	h.logger.Info("WebSocket client disconnected", zap.String("user_id", client.UserID))
}

func (h *Hub) SendToUser(userID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if clients, ok := h.clients[userID]; ok {
		for _, client := range clients {
			select {
			case client.Send <- data:
			default:
			}
		}
	}
}

func (h *Hub) BroadcastToChat(userIDs []string, data []byte) {
	if h.backend != nil {
		h.PublishEvent(userIDs, "", data)
		return
	}
	for _, uid := range userIDs {
		h.SendToUser(uid, data)
	}
}

// PublishEvent рассылает событие по кластеру (локальная доставка — через подписку).
func (h *Hub) PublishEvent(userIDs []string, msgType MessageType, data []byte) {
	ev, err := json.Marshal(clusterEvent{UserIDs: userIDs, Type: msgType, Data: data})
	if err != nil {
		h.logger.Warn("marshal cluster event", zap.Error(err))
		return
	}
	if err := h.backend.Publish(ev); err != nil {
		h.logger.Warn("publish cluster event", zap.Error(err))
	}
}

// PublishCluster публикует событие в кластерный канал без локальной доставки.
// Используется сервисами, которые не держат своих WS-клиентов, но хотят достучаться
// до подключённых пользователей через общий канал (например, call → chat hub).
func PublishCluster(backend ClusterBackend, userIDs []string, data []byte) error {
	ev, err := json.Marshal(clusterEvent{UserIDs: userIDs, Type: MsgMessage, Data: data})
	if err != nil {
		return err
	}
	return backend.Publish(ev)
}

func (h *Hub) HandleMessage(sender *Client, msgType MessageType, data []byte) {
	switch msgType {
	case MsgTyping:
		h.BroadcastToChat([]string{sender.UserID}, data)
	default:
		h.logger.Warn("unknown message type", zap.String("type", string(msgType)))
	}
}

func (h *Hub) GetUserCount(userID string) int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients[userID])
}

func (h *Hub) IsOnline(userID string) bool {
	return h.GetUserCount(userID) > 0
}
