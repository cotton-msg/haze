package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/cotton-msg/haze/backend/internal/models"
	"github.com/cotton-msg/haze/backend/internal/service"
	"github.com/cotton-msg/haze/backend/pkg/auth"
	"github.com/cotton-msg/haze/backend/pkg/utils"
	"github.com/go-chi/chi/v5"
)

type ChatHandler struct {
	svc *service.ChatService
}

func NewChatHandler(svc *service.ChatService) *ChatHandler {
	return &ChatHandler{svc: svc}
}

func (h *ChatHandler) CreateChat(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)

	var req service.CreateChatInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if len([]rune(req.Title)) > 255 {
		utils.ErrorResponse(w, http.StatusBadRequest, "title is too long (max 255)")
		return
	}
	if len(req.Members) > 500 {
		utils.ErrorResponse(w, http.StatusBadRequest, "too many members (max 500)")
		return
	}

	chat, err := h.svc.CreateChat(claims.UserID, req)
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, chat)
}

func (h *ChatHandler) ListChats(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)

	chats, err := h.svc.GetUserChats(claims.UserID)
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if chats == nil {
		chats = make([]*models.Chat, 0)
	}

	utils.SuccessResponse(w, chats)
}

// SavedChat возвращает (и при необходимости создаёт) личный чат «Избранное».
func (h *ChatHandler) SavedChat(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)

	chat, err := h.svc.SavedChat(claims.UserID)
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, chat)
}

// Presence возвращает онлайн-статус пользователя.
func (h *ChatHandler) Presence(w http.ResponseWriter, r *http.Request) {
	userID := chi.URLParam(r, "userId")

	utils.SuccessResponse(w, h.svc.GetPresence(userID))
}

func (h *ChatHandler) GetChat(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	chatID := chi.URLParam(r, "id")
	chat, err := h.svc.GetChat(chatID, claims.UserID)
	if err != nil {
		if errors.Is(err, service.ErrChatNotFound) {
			utils.ErrorResponse(w, http.StatusNotFound, "chat not found")
			return
		}
		if errors.Is(err, service.ErrNotMember) {
			utils.ErrorResponse(w, http.StatusForbidden, err.Error())
			return
		}
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, chat)
}

func (h *ChatHandler) GetMembers(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	chatID := chi.URLParam(r, "id")
	members, err := h.svc.GetMembers(chatID, claims.UserID)
	if err != nil {
		if errors.Is(err, service.ErrNotMember) {
			utils.ErrorResponse(w, http.StatusForbidden, err.Error())
			return
		}
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, members)
}

func (h *ChatHandler) SendMessage(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)

	var req service.SendMessageInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.ChatID == "" {
		req.ChatID = chi.URLParam(r, "id")
	}
	if req.Content == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "content is required")
		return
	}
	if len([]rune(req.Content)) > 4096 {
		utils.ErrorResponse(w, http.StatusBadRequest, "content is too long (max 4096)")
		return
	}

	msg, err := h.svc.SendMessage(claims.UserID, req)
	if err != nil {
		if errors.Is(err, service.ErrNotMember) {
			utils.ErrorResponse(w, http.StatusForbidden, err.Error())
			return
		}
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, msg)
}

func (h *ChatHandler) GetMessages(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	chatID := chi.URLParam(r, "id")

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))

	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	messages, err := h.svc.GetMessages(chatID, claims.UserID, limit, offset)
	if err != nil {
		if errors.Is(err, service.ErrNotMember) {
			utils.ErrorResponse(w, http.StatusForbidden, err.Error())
			return
		}
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, messages)
}

// SyncChat возвращает инкрементальные изменения чата после after_seq —
// для синхронизации всех устройств пользователя.
func (h *ChatHandler) SyncChat(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)

	chatID := r.URL.Query().Get("chat_id")
	afterSeq, _ := strconv.ParseInt(r.URL.Query().Get("after_seq"), 10, 64)
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	if chatID == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "chat_id is required")
		return
	}

	result, err := h.svc.SyncChat(chatID, claims.UserID, afterSeq, limit)
	if err != nil {
		if errors.Is(err, service.ErrNotMember) {
			utils.ErrorResponse(w, http.StatusForbidden, err.Error())
			return
		}
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, result)
}

// MarkRead помечает сообщения чата прочитанными (read receipts).
func (h *ChatHandler) MarkRead(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	chatID := chi.URLParam(r, "id")

	var req struct {
		MessageID string `json:"message_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.MessageID == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "message_id is required")
		return
	}

	if err := h.svc.MarkRead(chatID, claims.UserID, req.MessageID); err != nil {
		if errors.Is(err, service.ErrNotMember) {
			utils.ErrorResponse(w, http.StatusForbidden, err.Error())
			return
		}
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, map[string]string{"message": "marked as read"})
}

// Typing ретранслирует typing-событие остальным участникам чата.
func (h *ChatHandler) Typing(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	chatID := chi.URLParam(r, "id")

	if err := h.svc.BroadcastTyping(chatID, claims.UserID); err != nil {
		if errors.Is(err, service.ErrNotMember) {
			utils.ErrorResponse(w, http.StatusForbidden, err.Error())
			return
		}
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]string{"message": "typing broadcast"})
}

func (h *ChatHandler) EditMessage(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	msgID := chi.URLParam(r, "msgId")

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if len([]rune(req.Content)) > 4096 {
		utils.ErrorResponse(w, http.StatusBadRequest, "content is too long (max 4096)")
		return
	}

	msg, err := h.svc.EditMessage(claims.UserID, msgID, req.Content)
	if err != nil {
		if errors.Is(err, service.ErrNoPermission) {
			utils.ErrorResponse(w, http.StatusForbidden, err.Error())
			return
		}
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, msg)
}

func (h *ChatHandler) DeleteMessage(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	msgID := chi.URLParam(r, "msgId")

	if err := h.svc.DeleteMessage(claims.UserID, msgID); err != nil {
		if errors.Is(err, service.ErrNoPermission) {
			utils.ErrorResponse(w, http.StatusForbidden, err.Error())
			return
		}
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, map[string]string{"message": "deleted"})
}

func (h *ChatHandler) AddMember(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	chatID := chi.URLParam(r, "id")

	var req struct {
		UserID string `json:"user_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.svc.AddMember(chatID, claims.UserID, req.UserID); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, map[string]string{"message": "member added"})
}

func (h *ChatHandler) RemoveMember(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	chatID := chi.URLParam(r, "id")
	targetID := chi.URLParam(r, "userId")

	if err := h.svc.RemoveMember(chatID, claims.UserID, targetID); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, map[string]string{"message": "member removed"})
}
