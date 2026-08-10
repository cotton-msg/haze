package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

const (
	IndexUsers    = "users"
	IndexMessages = "messages"
)

type SearchService struct {
	client   meilisearch.ServiceManager
	usersIdx meilisearch.IndexManager
	msgsIdx  meilisearch.IndexManager
}

func NewSearchService(host, apiKey string) (*SearchService, error) {
	client := meilisearch.New(host, meilisearch.WithAPIKey(apiKey))
	if _, err := client.HealthWithContext(context.Background()); err != nil {
		return nil, fmt.Errorf("meilisearch unreachable: %w", err)
	}
	return &SearchService{
		client:   client,
		usersIdx: client.Index(IndexUsers),
		msgsIdx:  client.Index(IndexMessages),
	}, nil
}

func (s *SearchService) EnsureIndexes() error {
	for _, cfg := range []*meilisearch.IndexConfig{
		{Uid: IndexUsers, PrimaryKey: "id"},
		{Uid: IndexMessages, PrimaryKey: "id"},
	} {
		if _, err := s.client.CreateIndex(cfg); err != nil && !isIndexAlreadyExists(err) {
			return fmt.Errorf("create index %s: %w", cfg.Uid, err)
		}
	}

	if _, err := s.usersIdx.UpdateSettings(&meilisearch.Settings{
		SearchableAttributes: []string{"username", "display_name", "email", "phone"},
	}); err != nil {
		return fmt.Errorf("users settings: %w", err)
	}
	if _, err := s.msgsIdx.UpdateSettings(&meilisearch.Settings{
		SearchableAttributes: []string{"content"},
		SortableAttributes:   []string{"created_at"},
		FilterableAttributes: []string{"chat_id", "sender_id", "type", "created_at"},
	}); err != nil {
		return fmt.Errorf("messages settings: %w", err)
	}
	return nil
}

type UserDoc struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Email       string `json:"email"`
	Phone       string `json:"phone"`
	AvatarURL   string `json:"avatar_url"`
}

type MessageDoc struct {
	ID        string    `json:"id"`
	ChatID    string    `json:"chat_id"`
	SenderID  string    `json:"sender_id"`
	Content   string    `json:"content"`
	Type      string    `json:"type"`
	CreatedAt time.Time `json:"created_at"`
}

func (s *SearchService) IndexUsers(docs []UserDoc) error {
	if len(docs) == 0 {
		return nil
	}
	_, err := s.usersIdx.AddDocuments(docs, &meilisearch.DocumentOptions{})
	return err
}

func (s *SearchService) IndexMessages(docs []MessageDoc) error {
	if len(docs) == 0 {
		return nil
	}
	_, err := s.msgsIdx.AddDocuments(docs, &meilisearch.DocumentOptions{})
	return err
}

func (s *SearchService) DeleteUser(id string) error {
	_, err := s.usersIdx.DeleteDocument(id, &meilisearch.DocumentOptions{})
	return err
}

func (s *SearchService) DeleteMessage(id string) error {
	_, err := s.msgsIdx.DeleteDocument(id, &meilisearch.DocumentOptions{})
	return err
}

func (s *SearchService) SearchUsers(q string, limit, offset int64) ([]UserDoc, error) {
	resp, err := s.usersIdx.SearchWithContext(context.Background(), q, &meilisearch.SearchRequest{
		Limit:  limit,
		Offset: offset,
	})
	if err != nil {
		return nil, err
	}
	return decodeHits[UserDoc](resp.Hits)
}

type MessageSearchParams struct {
	Q        string
	ChatID   string
	SenderID string
	Type     string
	After    time.Time
	Before   time.Time
	Limit    int64
	Offset   int64
}

func (p MessageSearchParams) filter() string {
	var conds []string
	if p.ChatID != "" {
		conds = append(conds, fmt.Sprintf("chat_id = %q", p.ChatID))
	}
	if p.SenderID != "" {
		conds = append(conds, fmt.Sprintf("sender_id = %q", p.SenderID))
	}
	if p.Type != "" {
		conds = append(conds, fmt.Sprintf("type = %q", p.Type))
	}
	if !p.After.IsZero() {
		conds = append(conds, fmt.Sprintf("created_at > %d", p.After.Unix()))
	}
	if !p.Before.IsZero() {
		conds = append(conds, fmt.Sprintf("created_at < %d", p.Before.Unix()))
	}
	return strings.Join(conds, " AND ")
}

func (s *SearchService) SearchMessages(p MessageSearchParams) ([]MessageDoc, error) {
	req := &meilisearch.SearchRequest{
		Limit:  p.Limit,
		Offset: p.Offset,
		Sort:   []string{"created_at:desc"},
	}
	if f := p.filter(); f != "" {
		req.Filter = f
	}
	resp, err := s.msgsIdx.SearchWithContext(context.Background(), p.Q, req)
	if err != nil {
		return nil, err
	}
	return decodeHits[MessageDoc](resp.Hits)
}

func decodeHits[T any](hits meilisearch.Hits) ([]T, error) {
	data, err := json.Marshal(hits)
	if err != nil {
		return nil, err
	}
	var out []T
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func isIndexAlreadyExists(err error) bool {
	var meiliErr *meilisearch.Error
	if ok := errors.As(err, &meiliErr); ok {
		return meiliErr.MeilisearchApiError.Code == "index_already_exists"
	}
	return false
}
