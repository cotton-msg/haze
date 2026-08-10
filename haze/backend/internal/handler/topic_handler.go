package handler

import (
	"encoding/json"
	"net/http"

	"github.com/cotton-msg/haze/backend/internal/models"
	"github.com/cotton-msg/haze/backend/internal/repository"
	"github.com/cotton-msg/haze/backend/pkg/auth"
	"github.com/cotton-msg/haze/backend/pkg/utils"
	"github.com/go-chi/chi/v5"
)

// TopicHandler — CRUD тем (topics) в чатах.
type TopicHandler struct {
	topicRepo *repository.TopicRepository
	chatRepo  *repository.ChatRepository
}

func NewTopicHandler(topicRepo *repository.TopicRepository, chatRepo *repository.ChatRepository) *TopicHandler {
	return &TopicHandler{topicRepo: topicRepo, chatRepo: chatRepo}
}

func (h *TopicHandler) CreateTopic(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)

	var req struct {
		ChatID   string `json:"chat_id"`
		Title    string `json:"title"`
		IsPinned bool   `json:"is_pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ChatID == "" || req.Title == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "chat_id and title are required")
		return
	}
	if len([]rune(req.Title)) > 255 {
		utils.ErrorResponse(w, http.StatusBadRequest, "title is too long (max 255)")
		return
	}

	member, err := h.chatRepo.IsMember(req.ChatID, claims.UserID)
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !member {
		utils.ErrorResponse(w, http.StatusForbidden, "not a member")
		return
	}

	topic := &models.Topic{ChatID: req.ChatID, Title: req.Title, IsPinned: req.IsPinned}
	if err := h.topicRepo.Create(topic); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, topic)
}

func (h *TopicHandler) ListTopics(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	chatID := r.URL.Query().Get("chat_id")
	if chatID == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "chat_id is required")
		return
	}

	member, err := h.chatRepo.IsMember(chatID, claims.UserID)
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if !member {
		utils.ErrorResponse(w, http.StatusForbidden, "not a member")
		return
	}

	topics, err := h.topicRepo.FindByChatID(chatID)
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	if topics == nil {
		topics = make([]*models.Topic, 0)
	}
	utils.SuccessResponse(w, topics)
}

func (h *TopicHandler) UpdateTopic(w http.ResponseWriter, r *http.Request) {
	topicID := chi.URLParam(r, "id")

	var req struct {
		Title    string `json:"title"`
		IsPinned bool   `json:"is_pinned"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Title == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "title is required")
		return
	}
	if err := h.topicRepo.Update(&models.Topic{ID: topicID, Title: req.Title, IsPinned: req.IsPinned}); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]string{"message": "updated"})
}

func (h *TopicHandler) DeleteTopic(w http.ResponseWriter, r *http.Request) {
	topicID := chi.URLParam(r, "id")
	if err := h.topicRepo.Delete(topicID); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]string{"message": "deleted"})
}
