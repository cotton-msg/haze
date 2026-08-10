package handler

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cotton-msg/haze/backend/internal/models"
	"github.com/cotton-msg/haze/backend/internal/service"
	"github.com/go-chi/chi/v5"
)

// hTestCallRepo — in-memory реализация service.CallRepository для handler-тестов.
type hTestCallRepo struct {
	calls        map[string]*models.Call
	participants map[string][]string
}

func newHTestCallRepo() *hTestCallRepo {
	return &hTestCallRepo{
		calls:        map[string]*models.Call{},
		participants: map[string][]string{},
	}
}

func (m *hTestCallRepo) Create(call *models.Call) error {
	if call.ID == "" {
		call.ID = "call-1"
	}
	m.calls[call.ID] = call
	return nil
}

func (m *hTestCallRepo) FindByID(id string) (*models.Call, error) {
	if c, ok := m.calls[id]; ok {
		return c, nil
	}
	return nil, sql.ErrNoRows
}

func (m *hTestCallRepo) UpdateStatus(id string, status models.CallStatus) error {
	if c, ok := m.calls[id]; ok {
		c.Status = status
	}
	return nil
}

func (m *hTestCallRepo) GetHistory(userID string, limit, offset int) ([]*models.Call, error) {
	var out []*models.Call
	for _, c := range m.calls {
		if c.CallerID == userID || c.CalleeID == userID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *hTestCallRepo) AddParticipant(callID, userID string) error {
	m.participants[callID] = append(m.participants[callID], userID)
	return nil
}

func (m *hTestCallRepo) GetActiveCall(userID string) (*models.Call, error) {
	for _, c := range m.calls {
		if (c.CallerID == userID || c.CalleeID == userID) &&
			(c.Status == models.CallStatusRinging || c.Status == models.CallStatusActive) {
			return c, nil
		}
	}
	return nil, sql.ErrNoRows
}

// hTestCallHub — mock CallHub для handler-тестов.
type hTestCallHub struct {
	sent []byte
}

func (h *hTestCallHub) SendToUser(userID string, data []byte) { h.sent = data }

func newCallRouter(t *testing.T, callRepo service.CallRepository) http.Handler {
	t.Helper()
	svc := service.NewCallService(callRepo, &hTestCallHub{})
	h := NewCallHandler(svc)
	r := chi.NewRouter()
	r.Route("/calls", func(r chi.Router) {
		r.Post("/", h.StartCall)
		r.Post("/{id}/answer", h.AnswerCall)
		r.Post("/{id}/reject", h.RejectCall)
		r.Post("/{id}/end", h.EndCall)
		r.Post("/{id}/signal", h.Signaling)
		r.Get("/history", h.GetHistory)
	})
	return r
}

func TestCallHandlerStartCallOk(t *testing.T) {
	router := newCallRouter(t, newHTestCallRepo())

	body := `{"callee_id":"bob","type":"audio"}`
	req := httptest.NewRequest(http.MethodPost, "/calls/", bytes.NewBufferString(body))
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	resp := decodeResponse(t, w)
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got %v", resp["data"])
	}
	if data["callee_id"] != "bob" {
		t.Errorf("expected callee bob, got %v", data["callee_id"])
	}
}

func TestCallHandlerStartCallSelf(t *testing.T) {
	router := newCallRouter(t, newHTestCallRepo())

	body := `{"callee_id":"alice","type":"audio"}`
	req := httptest.NewRequest(http.MethodPost, "/calls/", bytes.NewBufferString(body))
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for self call, got %d", w.Code)
	}
}

func TestCallHandlerStartCallAlreadyActive(t *testing.T) {
	callRepo := newHTestCallRepo()
	callRepo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "alice", CalleeID: "bob", Status: models.CallStatusActive}

	router := newCallRouter(t, callRepo)

	body := `{"callee_id":"carol","type":"video"}`
	req := httptest.NewRequest(http.MethodPost, "/calls/", bytes.NewBufferString(body))
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400 for active call, got %d", w.Code)
	}
}

func TestCallHandlerStartCallInvalidBody(t *testing.T) {
	router := newCallRouter(t, newHTestCallRepo())

	req := httptest.NewRequest(http.MethodPost, "/calls/", bytes.NewBufferString("{bad"))
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCallHandlerAnswerCallOk(t *testing.T) {
	callRepo := newHTestCallRepo()
	callRepo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "alice", CalleeID: "bob", Status: models.CallStatusRinging}

	router := newCallRouter(t, callRepo)

	req := httptest.NewRequest(http.MethodPost, "/calls/call-1/answer", nil)
	req = withClaims(req, "bob")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if callRepo.calls["call-1"].Status != models.CallStatusActive {
		t.Errorf("expected call active, got %s", callRepo.calls["call-1"].Status)
	}
}

func TestCallHandlerAnswerCallNotFound(t *testing.T) {
	router := newCallRouter(t, newHTestCallRepo())

	req := httptest.NewRequest(http.MethodPost, "/calls/ghost/answer", nil)
	req = withClaims(req, "bob")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

func TestCallHandlerAnswerCallNotCallee(t *testing.T) {
	callRepo := newHTestCallRepo()
	callRepo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "alice", CalleeID: "bob", Status: models.CallStatusRinging}

	router := newCallRouter(t, callRepo)

	req := httptest.NewRequest(http.MethodPost, "/calls/call-1/answer", nil)
	req = withClaims(req, "carol")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestCallHandlerRejectCallOk(t *testing.T) {
	callRepo := newHTestCallRepo()
	callRepo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "alice", CalleeID: "bob", Status: models.CallStatusRinging}

	router := newCallRouter(t, callRepo)

	req := httptest.NewRequest(http.MethodPost, "/calls/call-1/reject", nil)
	req = withClaims(req, "bob")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if callRepo.calls["call-1"].Status != models.CallStatusRejected {
		t.Errorf("expected rejected, got %s", callRepo.calls["call-1"].Status)
	}
}

func TestCallHandlerRejectCallNotCallee(t *testing.T) {
	callRepo := newHTestCallRepo()
	callRepo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "alice", CalleeID: "bob", Status: models.CallStatusRinging}

	router := newCallRouter(t, callRepo)

	req := httptest.NewRequest(http.MethodPost, "/calls/call-1/reject", nil)
	req = withClaims(req, "carol")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestCallHandlerEndCallOk(t *testing.T) {
	callRepo := newHTestCallRepo()
	callRepo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "alice", CalleeID: "bob", Status: models.CallStatusActive}

	router := newCallRouter(t, callRepo)

	req := httptest.NewRequest(http.MethodPost, "/calls/call-1/end", nil)
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if callRepo.calls["call-1"].Status != models.CallStatusEnded {
		t.Errorf("expected ended, got %s", callRepo.calls["call-1"].Status)
	}
}

func TestCallHandlerEndCallNotParticipant(t *testing.T) {
	callRepo := newHTestCallRepo()
	callRepo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "alice", CalleeID: "bob", Status: models.CallStatusActive}

	router := newCallRouter(t, callRepo)

	req := httptest.NewRequest(http.MethodPost, "/calls/call-1/end", nil)
	req = withClaims(req, "carol")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestCallHandlerSignalingOk(t *testing.T) {
	callRepo := newHTestCallRepo()
	callRepo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "alice", CalleeID: "bob", Status: models.CallStatusActive}

	router := newCallRouter(t, callRepo)

	body := `{"type":"offer","sdp":"x"}`
	req := httptest.NewRequest(http.MethodPost, "/calls/call-1/signal", bytes.NewBufferString(body))
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
}

func TestCallHandlerSignalingMissingType(t *testing.T) {
	callRepo := newHTestCallRepo()
	callRepo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "alice", CalleeID: "bob", Status: models.CallStatusActive}

	router := newCallRouter(t, callRepo)

	body := `{"sdp":"x"}`
	req := httptest.NewRequest(http.MethodPost, "/calls/call-1/signal", bytes.NewBufferString(body))
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestCallHandlerSignalingNotParticipant(t *testing.T) {
	callRepo := newHTestCallRepo()
	callRepo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "alice", CalleeID: "bob", Status: models.CallStatusActive}

	router := newCallRouter(t, callRepo)

	body := `{"type":"offer","sdp":"x"}`
	req := httptest.NewRequest(http.MethodPost, "/calls/call-1/signal", bytes.NewBufferString(body))
	req = withClaims(req, "carol")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
}

func TestCallHandlerGetHistory(t *testing.T) {
	callRepo := newHTestCallRepo()
	callRepo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "alice", CalleeID: "bob", Status: models.CallStatusEnded}

	router := newCallRouter(t, callRepo)

	req := httptest.NewRequest(http.MethodGet, "/calls/history", nil)
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := decodeResponse(t, w)
	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatalf("expected data array, got %v", resp["data"])
	}
	if len(data) != 1 {
		t.Errorf("expected 1 call in history, got %d", len(data))
	}
}

func TestCallHandlerIceConfig(t *testing.T) {
	callRepo := newHTestCallRepo()
	callSvc := service.NewCallService(callRepo, &hTestCallHub{})
	callSvc.SetIceServers([]service.IceServer{
		{URLs: []string{"stun:stun.l.google.com:19302"}},
		{URLs: []string{"turn:coturn:3478"}, Username: "haze", Credential: "haze"},
	})
	h := NewCallHandler(callSvc)
	r := chi.NewRouter()
	r.Get("/ice-config", h.IceConfig)

	req := httptest.NewRequest(http.MethodGet, "/ice-config", nil)
	req = withClaims(req, "alice")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	resp := decodeResponse(t, w)
	data, _ := resp["data"].(map[string]interface{})
	servers, ok := data["servers"].([]interface{})
	if !ok {
		t.Fatalf("expected servers array, got %v", resp["data"])
	}
	if len(servers) != 2 {
		t.Errorf("expected 2 ice servers, got %d", len(servers))
	}
}

var _ = json.Marshal
