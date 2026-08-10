package ws

import (
	"testing"

	"go.uber.org/zap"
)

func TestNewHub(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	hub := NewHub(logger)

	if hub == nil {
		t.Fatal("hub should not be nil")
	}

	if hub.IsOnline("test-user") {
		t.Error("new user should not be online")
	}
}

func TestHubMethods(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	hub := NewHub(logger)

	if hub.GetUserCount("any") != 0 {
		t.Error("count should be 0")
	}

	hub.BroadcastToChat([]string{"a", "b"}, []byte(`{"test":1}`))
	hub.SendToUser("x", []byte(`{"test":1}`))
}

func TestMessageTypes(t *testing.T) {
	if MsgMessage != "message" {
		t.Error("MsgMessage should be 'message'")
	}
	if MsgTyping != "typing" {
		t.Error("MsgTyping should be 'typing'")
	}
	if MsgStatus != "status" {
		t.Error("MsgStatus should be 'status'")
	}
	if MsgRead != "read" {
		t.Error("MsgRead should be 'read'")
	}
}
