package repository

import (
	"context"
	"encoding/json"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
	"github.com/redis/go-redis/v9"
)

type CacheRepository struct {
	client *redis.Client
}

func NewCacheRepository(client *redis.Client) *CacheRepository {
	return &CacheRepository{client: client}
}

func (r *CacheRepository) SetSession(ctx context.Context, key string, session *models.Session, ttl time.Duration) error {
	data, err := json.Marshal(session)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, "session:"+key, data, ttl).Err()
}

func (r *CacheRepository) GetSession(ctx context.Context, key string) (*models.Session, error) {
	data, err := r.client.Get(ctx, "session:"+key).Bytes()
	if err != nil {
		return nil, err
	}
	var session models.Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, err
	}
	return &session, nil
}

func (r *CacheRepository) DelSession(ctx context.Context, key string) error {
	return r.client.Del(ctx, "session:"+key).Err()
}

func (r *CacheRepository) SetUser(ctx context.Context, userID string, user *models.User) error {
	data, err := json.Marshal(user)
	if err != nil {
		return err
	}
	return r.client.Set(ctx, "user:"+userID, data, 5*time.Minute).Err()
}

func (r *CacheRepository) GetUser(ctx context.Context, userID string) (*models.User, error) {
	data, err := r.client.Get(ctx, "user:"+userID).Bytes()
	if err != nil {
		return nil, err
	}
	var user models.User
	if err := json.Unmarshal(data, &user); err != nil {
		return nil, err
	}
	return &user, nil
}

func (r *CacheRepository) DelUser(ctx context.Context, userID string) error {
	return r.client.Del(ctx, "user:"+userID).Err()
}
