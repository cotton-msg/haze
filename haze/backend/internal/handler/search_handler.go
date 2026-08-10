package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
	"github.com/cotton-msg/haze/backend/internal/repository"
	"github.com/cotton-msg/haze/backend/internal/service"
	"github.com/cotton-msg/haze/backend/pkg/utils"
)

type SearchHandler struct {
	svc      *service.SearchService
	userRepo *repository.UserRepository
	msgRepo  *repository.MessageRepository
}

func NewSearchHandler(svc *service.SearchService, userRepo *repository.UserRepository, msgRepo *repository.MessageRepository) *SearchHandler {
	return &SearchHandler{svc: svc, userRepo: userRepo, msgRepo: msgRepo}
}

func (h *SearchHandler) SearchUsers(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		utils.SuccessResponse(w, map[string]interface{}{"query": q, "results": []interface{}{}})
		return
	}
	limit, offset := paginate(r)
	docs, err := h.svc.SearchUsers(q, int64(limit), int64(offset))
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]interface{}{"query": q, "results": docs})
}

func (h *SearchHandler) SearchMessages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		utils.SuccessResponse(w, map[string]interface{}{"query": q, "results": []interface{}{}})
		return
	}
	limit, offset := paginate(r)
	params := service.MessageSearchParams{
		Q:        q,
		ChatID:   r.URL.Query().Get("chat_id"),
		SenderID: r.URL.Query().Get("sender_id"),
		Type:     r.URL.Query().Get("type"),
		Limit:    int64(limit),
		Offset:   int64(offset),
	}
	if after := r.URL.Query().Get("after"); after != "" {
		t, err := time.Parse(time.RFC3339, after)
		if err != nil {
			utils.ErrorResponse(w, http.StatusBadRequest, "invalid after, use RFC3339")
			return
		}
		params.After = t
	}
	if before := r.URL.Query().Get("before"); before != "" {
		t, err := time.Parse(time.RFC3339, before)
		if err != nil {
			utils.ErrorResponse(w, http.StatusBadRequest, "invalid before, use RFC3339")
			return
		}
		params.Before = t
	}
	docs, err := h.svc.SearchMessages(params)
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]interface{}{"query": q, "results": docs})
}

func (h *SearchHandler) IndexUser(w http.ResponseWriter, r *http.Request) {
	var user models.User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}
	doc := service.UserDoc{
		ID: user.ID, Username: user.Username, DisplayName: user.DisplayName,
		Email: user.Email, Phone: user.Phone, AvatarURL: user.AvatarURL,
	}
	if err := h.svc.IndexUsers([]service.UserDoc{doc}); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]string{"status": "indexed"})
}

func (h *SearchHandler) IndexMessage(w http.ResponseWriter, r *http.Request) {
	var msg models.Message
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}
	doc := service.MessageDoc{
		ID: msg.ID, ChatID: msg.ChatID, SenderID: msg.SenderID,
		Content: msg.Content, Type: string(msg.Type), CreatedAt: msg.CreatedAt,
	}
	if err := h.svc.IndexMessages([]service.MessageDoc{doc}); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]string{"status": "indexed"})
}

func (h *SearchHandler) Sync(w http.ResponseWriter, r *http.Request) {
	if err := h.SyncAll(); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]interface{}{"status": "synced", "at": time.Now()})
}

func (h *SearchHandler) SyncAll() error {
	batch := 500
	offset := 0

	for {
		users, err := h.userRepo.ListAll(batch, offset)
		if err != nil {
			return err
		}
		if len(users) == 0 {
			break
		}
		docs := make([]service.UserDoc, 0, len(users))
		for _, u := range users {
			docs = append(docs, service.UserDoc{
				ID: u.ID, Username: u.Username, DisplayName: u.DisplayName,
				Email: u.Email, Phone: u.Phone, AvatarURL: u.AvatarURL,
			})
		}
		if err := h.svc.IndexUsers(docs); err != nil {
			return err
		}
		if len(users) < batch {
			break
		}
		offset += batch
	}

	offset = 0
	for {
		messages, err := h.msgRepo.ListAll(batch, offset)
		if err != nil {
			return err
		}
		if len(messages) == 0 {
			break
		}
		docs := make([]service.MessageDoc, 0, len(messages))
		for _, m := range messages {
			docs = append(docs, service.MessageDoc{
				ID: m.ID, ChatID: m.ChatID, SenderID: m.SenderID,
				Content: m.Content, Type: string(m.Type), CreatedAt: m.CreatedAt,
			})
		}
		if err := h.svc.IndexMessages(docs); err != nil {
			return err
		}
		if len(messages) < batch {
			break
		}
		offset += batch
	}

	return nil
}

func paginate(r *http.Request) (int, int) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}
	return limit, offset
}
