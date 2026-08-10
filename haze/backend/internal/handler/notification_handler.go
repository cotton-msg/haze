package handler

import (
	"encoding/json"
	"net/http"

	"github.com/cotton-msg/haze/backend/internal/repository"
	"github.com/cotton-msg/haze/backend/internal/service"
	"github.com/cotton-msg/haze/backend/pkg/auth"
	"github.com/cotton-msg/haze/backend/pkg/utils"
)

type NotificationHandler struct {
	pushRepo *repository.PushRepository
	pushSvc  *service.PushService
	userRepo *repository.UserRepository
}

func NewNotificationHandler(pushRepo *repository.PushRepository, pushSvc *service.PushService, userRepo *repository.UserRepository) *NotificationHandler {
	return &NotificationHandler{pushRepo: pushRepo, pushSvc: pushSvc, userRepo: userRepo}
}

type registerReq struct {
	Endpoint string `json:"endpoint"`
	P256DH   string `json:"p256dh"`
	Auth     string `json:"auth"`
}

func (h *NotificationHandler) Register(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)

	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Endpoint == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "endpoint is required")
		return
	}
	if len(req.Endpoint) > 2048 {
		utils.ErrorResponse(w, http.StatusBadRequest, "endpoint is too long")
		return
	}
	if len(req.P256DH) > 512 || len(req.Auth) > 512 {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid push keys")
		return
	}

	if err := h.pushRepo.Save(claims.UserID, req.Endpoint, req.P256DH, req.Auth); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]string{"status": "registered"})
}

func (h *NotificationHandler) Unregister(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)

	var req registerReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Endpoint == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "endpoint is required")
		return
	}

	if err := h.pushRepo.Delete(claims.UserID, req.Endpoint); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]string{"status": "unregistered"})
}

func (h *NotificationHandler) SaveSettings(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)

	var req struct {
		MutedChats []string `json:"muted_chats"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.pushRepo.SaveSettings(claims.UserID, req.MutedChats); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]string{"status": "saved"})
}

// SendRequest — payload push-уведомления (HTTP и очередь).
type SendRequest struct {
	Type      string   `json:"type"`
	MessageID string   `json:"message_id"`
	SenderID  string   `json:"sender_id"`
	ChatID    string   `json:"chat_id"`
	Content   string   `json:"content"`
	UserIDs   []string `json:"user_ids"`
}

func (h *NotificationHandler) Send(w http.ResponseWriter, r *http.Request) {
	var req SendRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}
	delivered := h.ProcessSend(req)
	utils.SuccessResponse(w, map[string]interface{}{"status": "sent", "delivered": delivered})
}

// Vapid отдаёт публичный VAPID-ключ для Web Push подписки.
func (h *NotificationHandler) Vapid(w http.ResponseWriter, r *http.Request) {
	key := h.pushSvc.PublicKey()
	if key == "" {
		utils.ErrorResponse(w, http.StatusServiceUnavailable, "vapid not configured")
		return
	}
	utils.SuccessResponse(w, map[string]string{"public_key": key})
}

// ProcessSend рассылает push-уведомления получателям (общая логика для HTTP и очереди).
func (h *NotificationHandler) ProcessSend(req SendRequest) int {
	if len(req.UserIDs) == 0 {
		return 0
	}
	total := 0
	for _, uid := range req.UserIDs {
		muted, err := h.pushRepo.GetMutedChats(uid)
		if err == nil && muted[req.ChatID] {
			continue
		}
		total += h.pushSvc.SendToUser(uid, service.PushPayload{
			Title: "Новое сообщение",
			Body:  req.Content,
			Data: struct {
				ChatID    string `json:"chat_id,omitempty"`
				MessageID string `json:"message_id,omitempty"`
			}{ChatID: req.ChatID, MessageID: req.MessageID},
		})
	}
	return total
}
