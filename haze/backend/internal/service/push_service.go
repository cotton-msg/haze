package service

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/cotton-msg/haze/backend/internal/repository"
)

type PushConfig struct {
	VAPIDPublicKey  string
	VAPIDPrivateKey string
	VAPIDSubject    string
}

// PushRepository — интерфейс push-подписок для сервисного слоя.
type PushRepository interface {
	Save(userID, endpoint, p256dh, auth string) error
	Delete(userID, endpoint string) error
	FindByUserID(userID string) ([]*repository.PushSubscription, error)
	GetMutedChats(userID string) (map[string]bool, error)
	SaveSettings(userID string, mutedChats []string) error
}

type PushService struct {
	cfg    PushConfig
	repo   PushRepository
	client *http.Client
	logger *log.Logger
}

// vapidKeyFile — место хранения сгенерированных ключей между рестартами,
// чтобы публичный ключ (VAPID endpoint) не менялся при каждом запуске.
const vapidKeyFile = "vapid_keys.json"

// ensureVAPIDKeys возвращает конфиг с заполненными ключами. Если ключи
// не заданы конфигом, они загружаются из файла, а при отсутствии — генерируются
// и сохраняются на диск.
func ensureVAPIDKeys(cfg PushConfig) PushConfig {
	if cfg.VAPIDPublicKey != "" && cfg.VAPIDPrivateKey != "" {
		return cfg
	}

	if data, err := os.ReadFile(vapidKeyFile); err == nil {
		var saved struct {
			PublicKey  string `json:"public_key"`
			PrivateKey string `json:"private_key"`
		}
		if json.Unmarshal(data, &saved) == nil && saved.PublicKey != "" && saved.PrivateKey != "" {
			cfg.VAPIDPublicKey = saved.PublicKey
			cfg.VAPIDPrivateKey = saved.PrivateKey
			log.Printf("loaded VAPID keys from %s", vapidKeyFile)
			return cfg
		}
	}

	pub, priv, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		log.Printf("failed to generate VAPID keys, push will be disabled: %v", err)
		return cfg
	}
	cfg.VAPIDPublicKey = pub
	cfg.VAPIDPrivateKey = priv
	log.Printf("generated new VAPID keys (public: %s)", pub)

	if data, err := json.Marshal(struct {
		PublicKey  string `json:"public_key"`
		PrivateKey string `json:"private_key"`
	}{pub, priv}); err == nil {
		if err := os.WriteFile(vapidKeyFile, data, 0600); err != nil {
			log.Printf("failed to persist VAPID keys to %s: %v", vapidKeyFile, err)
		}
	}
	return cfg
}

func NewPushService(cfg PushConfig, repo PushRepository) *PushService {
	cfg = ensureVAPIDKeys(cfg)
	if cfg.VAPIDSubject == "" {
		cfg.VAPIDSubject = "mailto:admin@haze.local"
	}
	return &PushService{
		cfg:    cfg,
		repo:   repo,
		client: &http.Client{Timeout: 10 * time.Second},
		logger: log.New(os.Stdout, "[push] ", log.LstdFlags),
	}
}

type PushPayload struct {
	Title string `json:"title"`
	Body  string `json:"body"`
	Data  struct {
		ChatID    string `json:"chat_id,omitempty"`
		MessageID string `json:"message_id,omitempty"`
	} `json:"data,omitempty"`
}

// PublicKey возвращает публичный VAPID-ключ для подписки на push.
func (s *PushService) PublicKey() string {
	return s.cfg.VAPIDPublicKey
}

// SendToUser отправляет push всем подпискам пользователя. Возвращает количество успешных.
func (s *PushService) SendToUser(userID string, payload PushPayload) int {
	subs, err := s.repo.FindByUserID(userID)
	if err != nil {
		s.logger.Printf("find subscriptions for %s: %v", userID, err)
		return 0
	}

	message, err := json.Marshal(payload)
	if err != nil {
		return 0
	}

	sent := 0
	for _, sub := range subs {
		ok := s.send(sub, message)
		if ok {
			sent++
		}
	}
	return sent
}

func (s *PushService) send(sub *repository.PushSubscription, message []byte) bool {
	if s.cfg.VAPIDPrivateKey == "" {
		return false
	}
	resp, err := webpush.SendNotificationWithContext(
		backgroundCtx(),
		message,
		&webpush.Subscription{
			Endpoint: sub.Endpoint,
			Keys: webpush.Keys{
				Auth:   sub.Auth,
				P256dh: sub.P256DH,
			},
		},
		&webpush.Options{
			Subscriber:      s.cfg.VAPIDSubject,
			VAPIDPublicKey:  s.cfg.VAPIDPublicKey,
			VAPIDPrivateKey: s.cfg.VAPIDPrivateKey,
			TTL:             60,
		},
	)
	if err != nil {
		s.logger.Printf("push to %s failed: %v", sub.Endpoint, err)
		return false
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != 410 && resp.StatusCode != 404 {
		s.logger.Printf("push to %s returned %d", sub.Endpoint, resp.StatusCode)
		return false
	}

	// 404/410 — подписка невалидна, удаляем.
	if resp.StatusCode == 410 || resp.StatusCode == 404 {
		if err := s.repo.Delete(sub.UserID, sub.Endpoint); err != nil {
			s.logger.Printf("delete stale subscription: %v", err)
		}
		return false
	}

	return true
}

func backgroundCtx() context.Context {
	return context.Background()
}
