package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/cotton-msg/haze/backend/internal/service"
	"github.com/cotton-msg/haze/backend/pkg/auth"
	"github.com/cotton-msg/haze/backend/pkg/utils"
	"github.com/go-chi/chi/v5"
)

type AuthHandler struct {
	svc *service.AuthService
}

func NewAuthHandler(svc *service.AuthService) *AuthHandler {
	return &AuthHandler{svc: svc}
}

func (h *AuthHandler) SSAAuthorize(w http.ResponseWriter, r *http.Request) {
	state := service.GenerateState()
	url := h.svc.GetSSAAuthorizeURL(state)
	http.Redirect(w, r, url, http.StatusFound)
}

func (h *AuthHandler) SSACallback(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")
	if code == "" || state == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "missing code or state")
		return
	}

	tokens, user, err := h.svc.HandleSSACallback(r.Context(), code, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, map[string]interface{}{
		"tokens": tokens,
		"user":   user,
	})
}

func (h *AuthHandler) Register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SSACode     string `json:"ssa_code"`
		Username    string `json:"username"`
		DisplayName string `json:"display_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SSACode == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "ssa_code is required")
		return
	}
	if len([]rune(req.Username)) > 32 {
		utils.ErrorResponse(w, http.StatusBadRequest, "username is too long (max 32)")
		return
	}
	if len([]rune(req.DisplayName)) > 100 {
		utils.ErrorResponse(w, http.StatusBadRequest, "display_name is too long (max 100)")
		return
	}

	tokens, user, err := h.svc.Register(r.Context(), req.SSACode, req.Username, req.DisplayName, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		if errors.Is(err, service.ErrUsernameTaken) {
			utils.ErrorResponse(w, http.StatusConflict, "username already taken")
			return
		}
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, map[string]interface{}{
		"tokens": tokens,
		"user":   user,
	})
}

func (h *AuthHandler) Login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SSACode string `json:"ssa_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.SSACode == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "ssa_code is required")
		return
	}

	tokens, user, err := h.svc.Login(r.Context(), req.SSACode, r.UserAgent(), r.RemoteAddr)
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, map[string]interface{}{
		"tokens": tokens,
		"user":   user,
	})
}

func (h *AuthHandler) Refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.RefreshToken == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	tokens, err := h.svc.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, service.ErrInvalidToken) || errors.Is(err, service.ErrSessionExpired) {
			utils.ErrorResponse(w, http.StatusUnauthorized, err.Error())
			return
		}
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, tokens)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	userID := claims.UserID

	var req struct {
		RefreshToken string `json:"refresh_token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.RefreshToken = ""
	}

	if err := h.svc.Logout(r.Context(), userID, req.RefreshToken); err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, map[string]string{"message": "logged out"})
}

func (h *AuthHandler) GetMe(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	userID := claims.UserID

	user, err := h.svc.GetMe(r.Context(), userID)
	if err != nil {
		utils.ErrorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	utils.SuccessResponse(w, user)
}

func (h *AuthHandler) UpdateProfile(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	userID := claims.UserID

	var req struct {
		DisplayName *string `json:"display_name"`
		Bio         *string `json:"bio"`
		AvatarURL   *string `json:"avatar_url"`
		StatusText  *string `json:"status_text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}

	updates := map[string]interface{}{}
	if req.DisplayName != nil {
		updates["display_name"] = *req.DisplayName
	}
	if req.Bio != nil {
		updates["bio"] = *req.Bio
	}
	if req.AvatarURL != nil {
		updates["avatar_url"] = *req.AvatarURL
	}
	if req.StatusText != nil {
		updates["status_text"] = *req.StatusText
	}

	user, err := h.svc.UpdateUser(r.Context(), userID, updates)
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, user)
}

func (h *AuthHandler) CheckUsername(w http.ResponseWriter, r *http.Request) {
	username := chi.URLParam(r, "username")
	if username == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "username is required")
		return
	}

	exists, err := h.svc.CheckUsername(username)
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, map[string]bool{"available": !exists})
}

func (h *AuthHandler) UpdateUsername(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	userID := claims.UserID

	var req struct {
		Username string `json:"username"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Username == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "username is required")
		return
	}
	if len([]rune(req.Username)) > 32 {
		utils.ErrorResponse(w, http.StatusBadRequest, "username is too long (max 32)")
		return
	}

	err := h.svc.UpdateUsername(context.Background(), userID, req.Username)
	if err != nil {
		if errors.Is(err, service.ErrUsernameTaken) {
			utils.ErrorResponse(w, http.StatusConflict, "username already taken")
			return
		}
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, map[string]string{"username": req.Username})
}
