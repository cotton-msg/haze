package ws

import (
	"encoding/json"
	"net/http"

	"github.com/cotton-msg/haze/backend/pkg/auth"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type Handler struct {
	hub    *Hub
	logger *zap.Logger
}

func NewWSHandler(hub *Hub, logger *zap.Logger) *Handler {
	return &Handler{hub: hub, logger: logger}
}

func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	claims, ok := r.Context().Value(auth.ClaimsKey).(*auth.Claims)
	if !ok || claims == nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.Warn("WS upgrade failed", zap.Error(err))
		return
	}

	client := NewClient(claims.UserID, conn, h.hub, h.logger)
	h.hub.Register(client)

	go client.WritePump()
	go client.ReadPump()
}

func WriteJSON(conn *websocket.Conn, msgType MessageType, payload interface{}) error {
	data, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	msg := Message{
		Type:    msgType,
		Payload: data,
	}
	return conn.WriteJSON(msg)
}
