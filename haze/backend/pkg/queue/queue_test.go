package queue

import (
	"encoding/json"
	"testing"
)

func TestMessageJSON(t *testing.T) {
	msg := Message{
		Type:   "push",
		Target: "user-1",
		Body:   json.RawMessage(`{"chat_id":"c1"}`),
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Message
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if decoded.Type != "push" || decoded.Target != "user-1" {
		t.Errorf("roundtrip mismatch: %+v", decoded)
	}
	var body map[string]string
	if err := json.Unmarshal(decoded.Body, &body); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if body["chat_id"] != "c1" {
		t.Errorf("expected body.chat_id c1, got %v", body)
	}
}
