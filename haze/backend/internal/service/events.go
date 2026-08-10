package service

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
	"github.com/cotton-msg/haze/backend/pkg/queue"
)

// maxAttempts — число попыток доставки события (включая первую).
const maxAttempts = 3

// backoff — экспоненциальная задержка между попытками.
func backoff(attempt int) time.Duration {
	return time.Duration(attempt*attempt) * 200 * time.Millisecond
}

// Indexer отправляет события индексации в search-сервис.
type Indexer interface {
	IndexUser(u *models.User)
	IndexMessage(m *models.Message)
}

// PushNotifier уведомляет notification-сервис о новых сообщениях.
type PushNotifier interface {
	NotifyMessage(m *models.Message, recipientIDs []string)
}

type HTTPIndexer struct {
	baseURL       string
	internalToken string
	client        *http.Client
	logger        *log.Logger
}

func NewHTTPIndexer(baseURL string) *HTTPIndexer {
	return &HTTPIndexer{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 3 * time.Second},
		logger:  log.New(os.Stdout, "[indexer] ", log.LstdFlags),
	}
}

// SetInternalToken задаёт токен для service-to-service вызовов индексации.
func (h *HTTPIndexer) SetInternalToken(token string) *HTTPIndexer {
	h.internalToken = token
	return h
}

func (h *HTTPIndexer) IndexUser(u *models.User) {
	doc := UserDoc{
		ID: u.ID, Username: u.Username, DisplayName: u.DisplayName,
		Email: u.Email, Phone: u.Phone, AvatarURL: u.AvatarURL,
	}
	h.post("/search/index/user", doc)
}

func (h *HTTPIndexer) IndexMessage(m *models.Message) {
	doc := MessageDoc{
		ID: m.ID, ChatID: m.ChatID, SenderID: m.SenderID,
		Content: m.Content, Type: string(m.Type), CreatedAt: m.CreatedAt,
	}
	h.post("/search/index/message", doc)
}

func (h *HTTPIndexer) post(path string, body interface{}) {
	if h.baseURL == "" {
		return
	}
	data, err := json.Marshal(body)
	if err != nil {
		h.logger.Printf("marshal %s: %v", path, err)
		return
	}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			time.Sleep(backoff(attempt))
		}
		req, err := http.NewRequest(http.MethodPost, h.baseURL+path, bytes.NewReader(data))
		if err != nil {
			h.logger.Printf("new request %s: %v", path, err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		if h.internalToken != "" {
			req.Header.Set("X-Internal-Token", h.internalToken)
		}
		resp, err := h.client.Do(req)
		if err != nil {
			h.logger.Printf("POST %s attempt %d: %v", path, attempt, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return
		}
		h.logger.Printf("POST %s attempt %d: status %d", path, attempt, resp.StatusCode)
	}
	h.logger.Printf("POST %s: failed after %d attempts", path, maxAttempts)
}

type HTTPPushNotifier struct {
	baseURL string
	client  *http.Client
	logger  *log.Logger
}

func NewHTTPPushNotifier(baseURL string) *HTTPPushNotifier {
	return &HTTPPushNotifier{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 3 * time.Second},
		logger:  log.New(os.Stdout, "[notifier] ", log.LstdFlags),
	}
}

func (n *HTTPPushNotifier) NotifyMessage(m *models.Message, recipientIDs []string) {
	if n.baseURL == "" || len(recipientIDs) == 0 {
		return
	}
	body := map[string]interface{}{
		"type":       "new_message",
		"message_id": m.ID,
		"sender_id":  m.SenderID,
		"chat_id":    m.ChatID,
		"content":    m.Content,
		"user_ids":   recipientIDs,
	}
	data, err := json.Marshal(body)
	if err != nil {
		n.logger.Printf("marshal: %v", err)
		return
	}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			time.Sleep(backoff(attempt))
		}
		req, err := http.NewRequest(http.MethodPost, n.baseURL+"/notifications/send", bytes.NewReader(data))
		if err != nil {
			n.logger.Printf("new request: %v", err)
			return
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := n.client.Do(req)
		if err != nil {
			n.logger.Printf("POST /notifications/send attempt %d: %v", attempt, err)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return
		}
		n.logger.Printf("POST /notifications/send attempt %d: status %d", attempt, resp.StatusCode)
	}
	n.logger.Printf("POST /notifications/send: failed after %d attempts", maxAttempts)
}

// RedisPushNotifier публикует события новых сообщений в Redis Stream
// вместо прямого HTTP-вызова (масштабируемая очередь).
type RedisPushNotifier struct {
	producer *queue.Producer
	logger   *log.Logger
}

func NewRedisPushNotifier(producer *queue.Producer) *RedisPushNotifier {
	return &RedisPushNotifier{
		producer: producer,
		logger:   log.New(os.Stdout, "[notifier] ", log.LstdFlags),
	}
}

func (n *RedisPushNotifier) NotifyMessage(m *models.Message, recipientIDs []string) {
	if n.producer == nil || len(recipientIDs) == 0 {
		return
	}
	body, err := json.Marshal(map[string]interface{}{
		"message_id": m.ID,
		"sender_id":  m.SenderID,
		"chat_id":    m.ChatID,
		"content":    m.Content,
		"user_ids":   recipientIDs,
	})
	if err != nil {
		n.logger.Printf("marshal: %v", err)
		return
	}
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 {
			time.Sleep(backoff(attempt))
		}
		if err := n.producer.Enqueue(queue.Message{
			Type:   "new_message",
			Target: "notification",
			Body:   body,
		}); err != nil {
			n.logger.Printf("enqueue new_message attempt %d: %v", attempt, err)
			continue
		}
		return
	}
	n.logger.Printf("enqueue new_message: failed after %d attempts", maxAttempts)
}
