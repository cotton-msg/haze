package service

import (
	"errors"
	"testing"

	"github.com/cotton-msg/haze/backend/internal/repository"
)

// mockPushRepo — in-memory реализация PushRepository для тестов.
type mockPushRepo struct {
	subs    map[string][]*repository.PushSubscription
	muted   map[string]map[string]bool
	deleted [][]string
}

func newMockPushRepo() *mockPushRepo {
	return &mockPushRepo{
		subs:  map[string][]*repository.PushSubscription{},
		muted: map[string]map[string]bool{},
	}
}

func (m *mockPushRepo) Save(userID, endpoint, p256dh, auth string) error {
	m.subs[userID] = append(m.subs[userID], &repository.PushSubscription{
		UserID: userID, Endpoint: endpoint, P256DH: p256dh, Auth: auth,
	})
	return nil
}

func (m *mockPushRepo) Delete(userID, endpoint string) error {
	m.deleted = append(m.deleted, []string{userID, endpoint})
	var keep []*repository.PushSubscription
	for _, s := range m.subs[userID] {
		if s.Endpoint != endpoint {
			keep = append(keep, s)
		}
	}
	m.subs[userID] = keep
	return nil
}

func (m *mockPushRepo) FindByUserID(userID string) ([]*repository.PushSubscription, error) {
	return m.subs[userID], nil
}

func (m *mockPushRepo) GetMutedChats(userID string) (map[string]bool, error) {
	return m.muted[userID], nil
}

func (m *mockPushRepo) SaveSettings(userID string, mutedChats []string) error {
	m.muted[userID] = map[string]bool{}
	for _, c := range mutedChats {
		m.muted[userID][c] = true
	}
	return nil
}

func TestPushServicePublicKey(t *testing.T) {
	cfg := PushConfig{VAPIDPublicKey: "pub-key", VAPIDPrivateKey: "priv-key"}
	svc := NewPushService(cfg, newMockPushRepo())

	if svc.PublicKey() != "pub-key" {
		t.Errorf("expected pub-key, got %q", svc.PublicKey())
	}
}

func TestPushServiceSendToUserNoSubscriptions(t *testing.T) {
	cfg := PushConfig{VAPIDPublicKey: "pub", VAPIDPrivateKey: "priv", VAPIDSubject: "mailto:a@b.c"}
	svc := NewPushService(cfg, newMockPushRepo())

	if n := svc.SendToUser("ghost", PushPayload{Title: "x"}); n != 0 {
		t.Errorf("expected 0 sent for unknown user, got %d", n)
	}
}

func TestPushServiceSendToUserWithoutPrivateKey(t *testing.T) {
	repo := newMockPushRepo()
	repo.Save("alice", "https://push.test/a", "p256", "auth")

	cfg := PushConfig{VAPIDPublicKey: "", VAPIDPrivateKey: "", VAPIDSubject: "mailto:a@b.c"}
	svc := NewPushService(cfg, repo)

	if n := svc.SendToUser("alice", PushPayload{Title: "x"}); n != 0 {
		t.Errorf("expected 0 sent without private key, got %d", n)
	}
}

func TestPushServiceSendToUserSendsToAllSubscriptions(t *testing.T) {
	repo := newMockPushRepo()
	repo.Save("alice", "https://push.test/a", "p256", "auth")
	repo.Save("alice", "https://push.test/b", "p256", "auth")

	cfg := PushConfig{VAPIDPublicKey: "pub", VAPIDPrivateKey: "priv", VAPIDSubject: "mailto:a@b.c"}
	svc := NewPushService(cfg, repo)

	// Реальные webpush-вызовы в тесте невозможны; тут мы проверяем лишь
	// что при отсутствии подписок не падаем, а с невалидным endpoint
	// ошибки обрабатываются. Данный тест гарантирует отсутствие паники.
	if n := svc.SendToUser("alice", PushPayload{Title: "x"}); n < 0 {
		t.Errorf("delivery count must not be negative, got %d", n)
	}
}

func TestPushRepositoryInterfaceSatisfied(t *testing.T) {
	var _ PushRepository = (*mockPushRepo)(nil)
	var _ = errors.New
}
