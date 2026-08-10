package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/cotton-msg/haze/backend/internal/service"
	"github.com/cotton-msg/haze/backend/pkg/auth"
	"github.com/cotton-msg/haze/backend/pkg/utils"
	"github.com/go-chi/chi/v5"
)

type CallHandler struct {
	svc *service.CallService
}

func NewCallHandler(svc *service.CallService) *CallHandler {
	return &CallHandler{svc: svc}
}

func (h *CallHandler) StartCall(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)

	var req service.StartCallInput
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid body")
		return
	}

	call, err := h.svc.StartCall(claims.UserID, req)
	if err != nil {
		if errors.Is(err, service.ErrSelfCall) || errors.Is(err, service.ErrActiveCall) {
			utils.ErrorResponse(w, http.StatusBadRequest, err.Error())
			return
		}
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, call)
}

func (h *CallHandler) AnswerCall(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	callID := chi.URLParam(r, "id")

	call, err := h.svc.AnswerCall(callID, claims.UserID)
	if err != nil {
		writeCallError(w, err)
		return
	}

	utils.SuccessResponse(w, call)
}

func (h *CallHandler) RejectCall(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	callID := chi.URLParam(r, "id")

	call, err := h.svc.RejectCall(callID, claims.UserID)
	if err != nil {
		writeCallError(w, err)
		return
	}

	utils.SuccessResponse(w, call)
}

func (h *CallHandler) EndCall(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	callID := chi.URLParam(r, "id")

	call, err := h.svc.EndCall(callID, claims.UserID)
	if err != nil {
		writeCallError(w, err)
		return
	}

	utils.SuccessResponse(w, call)
}

// writeCallError отображает ошибки сервиса звонков на корректные HTTP-статусы.
func writeCallError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrCallNotFound):
		utils.ErrorResponse(w, http.StatusNotFound, err.Error())
	case errors.Is(err, service.ErrNoPermission), errors.Is(err, service.ErrNotParticipant):
		utils.ErrorResponse(w, http.StatusForbidden, err.Error())
	default:
		utils.ErrorResponse(w, http.StatusBadRequest, err.Error())
	}
}

func (h *CallHandler) GetHistory(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}

	history, err := h.svc.GetHistory(claims.UserID, limit, offset)
	if err != nil {
		utils.ErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	utils.SuccessResponse(w, history)
}

// IceConfig отдаёт список STUN/TURN серверов для WebRTC.
func (h *CallHandler) IceConfig(w http.ResponseWriter, r *http.Request) {
	utils.SuccessResponse(w, map[string]interface{}{"servers": h.svc.IceServers()})
}

func (h *CallHandler) Signaling(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	callID := chi.URLParam(r, "id")

	var payload map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		utils.ErrorResponse(w, http.StatusBadRequest, "invalid body")
		return
	}

	msgType, _ := payload["type"].(string)
	if msgType == "" {
		utils.ErrorResponse(w, http.StatusBadRequest, "missing type")
		return
	}

	if err := h.svc.HandleSignaling(callID, claims.UserID, msgType, payload); err != nil {
		writeCallError(w, err)
		return
	}

	utils.SuccessResponse(w, map[string]string{"status": "relayed"})
}
