package service

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// presenceOnlineTTL — TTL маркера онлайн: держим чуть больше pingPeriod,
// чтобы heartbeat от pong не давал ложных оффлайнов.
const presenceOnlineTTL = 90 * time.Second

// Presence — статус пользователя для API.
type Presence struct {
	Online    bool      `json:"online"`
	LastSeen  time.Time `json:"last_seen_at"`
}

// PresenceService — отслеживание онлайн/оффлайн через Redis.
// Реализует ws.PresenceHook и используется для чтения статусов.
type PresenceService struct {
	client redis.Cmdable
	prefix string
}

func NewPresenceService(client redis.Cmdable, prefix string) *PresenceService {
	return &PresenceService{client: client, prefix: prefix}
}

func (p *PresenceService) onlineKey(userID string) string { return p.prefix + ":online:" + userID }
func (p *PresenceService) lastKey(userID string) string   { return p.prefix + ":last:" + userID }

func (p *PresenceService) Online(userID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	pipe := p.client.TxPipeline()
	pipe.Set(ctx, p.onlineKey(userID), "1", presenceOnlineTTL)
	pipe.Set(ctx, p.lastKey(userID), time.Now().Unix(), 0)
	pipe.Exec(ctx)
}

func (p *PresenceService) Offline(userID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	p.client.Del(ctx, p.onlineKey(userID))
}

func (p *PresenceService) Heartbeat(userID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	p.client.Expire(ctx, p.onlineKey(userID), presenceOnlineTTL)
}

// Get возвращает статус пользователя.
func (p *PresenceService) Get(userID string) Presence {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	exists, err := p.client.Exists(ctx, p.onlineKey(userID)).Result()
	if err != nil {
		return Presence{}
	}
	last, err := p.client.Get(ctx, p.lastKey(userID)).Int64()
	if err != nil {
		last = 0
	}
	return Presence{
		Online:   exists > 0,
		LastSeen: time.Unix(last, 0),
	}
}
