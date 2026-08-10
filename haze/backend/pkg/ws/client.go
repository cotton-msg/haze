package ws

import (
	"encoding/json"
	"time"

	"github.com/cotton-msg/haze/backend/pkg/metrics"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	writeWait      = 10 * time.Second
	pongWait       = 30 * time.Second
	pingPeriod     = (pongWait * 9) / 10
	maxMessageSize = 65536
)

type Client struct {
	UserID string
	Conn   *websocket.Conn
	Send   chan []byte
	hub    *Hub
	logger *zap.Logger
}

func NewClient(userID string, conn *websocket.Conn, hub *Hub, logger *zap.Logger) *Client {
	return &Client{
		UserID: userID,
		Conn:   conn,
		Send:   make(chan []byte, 256),
		hub:    hub,
		logger: logger,
	}
}

func (c *Client) ReadPump() {
	defer func() {
		c.hub.Unregister(c)
		c.Conn.Close()
	}()

	c.Conn.SetReadLimit(maxMessageSize)
	c.Conn.SetReadDeadline(time.Now().Add(pongWait))
	c.Conn.SetPongHandler(func(string) error {
		c.Conn.SetReadDeadline(time.Now().Add(pongWait))
		if c.hub.presence != nil {
			c.hub.presence.Heartbeat(c.UserID)
		}
		return nil
	})

	for {
		_, data, err := c.Conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseNormalClosure) {
				c.logger.Warn("WS read error", zap.Error(err))
			}
			break
		}

		if c.hub.presence != nil {
			c.hub.presence.Heartbeat(c.UserID)
		}

		var msg Message
		if err := json.Unmarshal(data, &msg); err != nil {
			c.logger.Warn("invalid WS message", zap.Error(err))
			continue
		}

		msgBytes, _ := json.Marshal(msg)
		metrics.WSMessagesTotal.WithLabelValues(c.hub.service, "in").Inc()
		c.hub.HandleMessage(c, msg.Type, msgBytes)
	}
}

func (c *Client) WritePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.Conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.Send:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				c.Conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			if err := c.Conn.WriteMessage(websocket.TextMessage, message); err != nil {
				return
			}
			metrics.WSMessagesTotal.WithLabelValues(c.hub.service, "out").Inc()
		case <-ticker.C:
			c.Conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.Conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}
