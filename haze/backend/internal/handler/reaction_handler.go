package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
	"github.com/cotton-msg/haze/backend/internal/repository"
	"github.com/cotton-msg/haze/backend/pkg/auth"
	"github.com/cotton-msg/haze/backend/pkg/utils"
	"github.com/go-chi/chi/v5"
)

type ReactionHandler struct {
	reactionRepo *repository.ReactionRepository
	stickerRepo  *repository.StickerRepository
	chatRepo     *repository.ChatRepository
	msgRepo      *repository.MessageRepository
}

func NewReactionHandler(reactionRepo *repository.ReactionRepository, stickerRepo *repository.StickerRepository, chatRepo *repository.ChatRepository, msgRepo *repository.MessageRepository) *ReactionHandler {
	return &ReactionHandler{reactionRepo: reactionRepo, stickerRepo: stickerRepo, chatRepo: chatRepo, msgRepo: msgRepo}
}

// checkMessageAccess проверяет, что сообщение существует и пользователь — участник его чата.
func (h *ReactionHandler) checkMessageAccess(msgID, userID string) error {
	msg, err := h.msgRepo.FindByID(msgID)
	if err != nil {
		return err
	}
	isMember, err := h.chatRepo.IsMember(msg.ChatID, userID)
	if err != nil {
		return err
	}
	if !isMember {
		return errNotMember
	}
	return nil
}

var errNotMember = errors.New("user is not a member of this chat")

func (h *ReactionHandler) AddReaction(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	msgID := chi.URLParam(r, "msgId")

	var req struct {
		Emoji string `json:"emoji"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid body")
		return
	}
	if req.Emoji == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "emoji is required")
		return
	}

	if err := h.checkMessageAccess(msgID, claims.UserID); err != nil {
		if errors.Is(err, errNotMember) {
			utils.ErrorResponse(w, http.StatusForbidden, err.Error())
			return
		}
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	reaction := &models.Reaction{
		MessageID: msgID,
		UserID:    claims.UserID,
		Emoji:     req.Emoji,
		CreatedAt: time.Now(),
	}

	if err := h.reactionRepo.Upsert(reaction); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, reaction)
}

func (h *ReactionHandler) RemoveReaction(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	msgID := chi.URLParam(r, "msgId")
	emoji := chi.URLParam(r, "emoji")

	if err := h.checkMessageAccess(msgID, claims.UserID); err != nil {
		if errors.Is(err, errNotMember) {
			utils.ErrorResponse(w, http.StatusForbidden, err.Error())
			return
		}
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	if err := h.reactionRepo.Delete(msgID, claims.UserID, emoji); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, map[string]string{"message": "removed"})
}

func (h *ReactionHandler) GetReactions(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	msgID := chi.URLParam(r, "msgId")

	if err := h.checkMessageAccess(msgID, claims.UserID); err != nil {
		if errors.Is(err, errNotMember) {
			utils.ErrorResponse(w, http.StatusForbidden, err.Error())
			return
		}
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Агрегированные счётчики — то, что нужно UI для бейджей реакций.
	counts, err := h.reactionRepo.FindByMessageIDAggregated(msgID)
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, counts)
}

// isAdmin — проверка админ-роли для управления контентом.
func isAdmin(claims *auth.Claims) bool {
	return claims.Role == "owner" || claims.Role == "developer" || claims.Role == "admin" || claims.Role == "moderator"
}

func (h *ReactionHandler) CreateStickerPack(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	if !isAdmin(claims) {
		utils.ErrorResponse(w, http.StatusForbidden, "forbidden")
		return
	}
	var pack models.StickerPack
	if err := json.NewDecoder(r.Body).Decode(&pack); err != nil || pack.Name == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "name is required")
		return
	}
	if err := h.stickerRepo.CreatePack(&pack); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, pack)
}

func (h *ReactionHandler) UpdateStickerPack(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	if !isAdmin(claims) {
		utils.ErrorResponse(w, http.StatusForbidden, "forbidden")
		return
	}
	packID := chi.URLParam(r, "packId")
	var pack models.StickerPack
	if err := json.NewDecoder(r.Body).Decode(&pack); err != nil || pack.Name == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "name is required")
		return
	}
	pack.ID = packID
	if err := h.stickerRepo.UpdatePack(&pack); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, pack)
}

func (h *ReactionHandler) DeleteStickerPack(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	if !isAdmin(claims) {
		utils.ErrorResponse(w, http.StatusForbidden, "forbidden")
		return
	}
	packID := chi.URLParam(r, "packId")
	if err := h.stickerRepo.DeletePack(packID); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]string{"message": "pack deleted"})
}

func (h *ReactionHandler) AddSticker(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	if !isAdmin(claims) {
		utils.ErrorResponse(w, http.StatusForbidden, "forbidden")
		return
	}
	packID := chi.URLParam(r, "packId")
	var sticker models.Sticker
	if err := json.NewDecoder(r.Body).Decode(&sticker); err != nil || sticker.Name == "" || sticker.ImageURL == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "name and image_url are required")
		return
	}
	sticker.PackID = packID
	if err := h.stickerRepo.CreateSticker(&sticker); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, sticker)
}

func (h *ReactionHandler) DeleteSticker(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	if !isAdmin(claims) {
		utils.ErrorResponse(w, http.StatusForbidden, "forbidden")
		return
	}
	stickerID := chi.URLParam(r, "stickerId")
	if err := h.stickerRepo.DeleteSticker(stickerID); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, map[string]string{"message": "sticker deleted"})
}

func (h *ReactionHandler) GetStickerPacks(w http.ResponseWriter, r *http.Request) {
	packs, err := h.stickerRepo.GetPacks()
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, packs)
}

func (h *ReactionHandler) GetStickers(w http.ResponseWriter, r *http.Request) {
	packID := chi.URLParam(r, "packId")
	stickers, err := h.stickerRepo.GetByPack(packID)
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}
	utils.SuccessResponse(w, stickers)
}
