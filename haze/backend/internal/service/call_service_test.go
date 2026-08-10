package service

import (
	"database/sql"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
)

// mockCallRepo — in-memory реализация CallRepository для тестов.
type mockCallRepo struct {
	mu           sync.Mutex
	calls        map[string]*models.Call
	participants map[string][]string
	created      *models.Call
	lastStatus   map[string]models.CallStatus
}

func newMockCallRepo() *mockCallRepo {
	return &mockCallRepo{
		calls:        map[string]*models.Call{},
		participants: map[string][]string{},
		lastStatus:   map[string]models.CallStatus{},
	}
}

func (m *mockCallRepo) Create(call *models.Call) error {
	if call.ID == "" {
		call.ID = "call-1"
	}
	m.calls[call.ID] = call
	m.created = call
	return nil
}

func (m *mockCallRepo) FindByID(id string) (*models.Call, error) {
	if c, ok := m.calls[id]; ok {
		return c, nil
	}
	return nil, errors.New("sql: no rows")
}

func (m *mockCallRepo) UpdateStatus(id string, status models.CallStatus) error {
	if c, ok := m.calls[id]; ok {
		c.Status = status
		m.lastStatus[id] = status
	}
	return nil
}

func (m *mockCallRepo) GetHistory(userID string, limit, offset int) ([]*models.Call, error) {
	var out []*models.Call
	for _, c := range m.calls {
		if c.CallerID == userID || c.CalleeID == userID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *mockCallRepo) AddParticipant(callID, userID string) error {
	m.participants[callID] = append(m.participants[callID], userID)
	return nil
}

func (m *mockCallRepo) GetActiveCall(userID string) (*models.Call, error) {
	for _, c := range m.calls {
		if (c.CallerID == userID || c.CalleeID == userID) &&
			(c.Status == models.CallStatusRinging || c.Status == models.CallStatusActive) {
			return c, nil
		}
	}
	return nil, sql.ErrNoRows
}

type mockCallHub struct {
	mu     sync.Mutex
	sentTo map[string][][]byte
}

func newMockCallHub() *mockCallHub {
	return &mockCallHub{sentTo: map[string][][]byte{}}
}

func (m *mockCallHub) SendToUser(userID string, data []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sentTo[userID] = append(m.sentTo[userID], data)
}

func (m *mockCallHub) count(userID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sentTo[userID])
}

func newTestCallService(repo *mockCallRepo, hub *mockCallHub) *CallService {
	return NewCallService(repo, hub)
}

func TestStartCallSendsIncomingEvent(t *testing.T) {
	repo := newMockCallRepo()
	hub := newMockCallHub()
	svc := newTestCallService(repo, hub)

	call, err := svc.StartCall("caller-1", StartCallInput{CalleeID: "callee-1", Type: models.CallTypeVideo})
	if err != nil {
		t.Fatalf("StartCall: %v", err)
	}
	if call.Status != models.CallStatusRinging {
		t.Errorf("expected ringing, got %s", call.Status)
	}
	if hub.count("callee-1") != 1 {
		t.Errorf("expected incoming event to callee, got %d", hub.count("callee-1"))
	}
}

func TestStartCallSelfRejected(t *testing.T) {
	repo := newMockCallRepo()
	hub := newMockCallHub()
	svc := newTestCallService(repo, hub)

	_, err := svc.StartCall("user-1", StartCallInput{CalleeID: "user-1"})
	if !errors.Is(err, ErrSelfCall) {
		t.Fatalf("expected ErrSelfCall, got %v", err)
	}
}

func TestStartCallActiveRejected(t *testing.T) {
	repo := newMockCallRepo()
	repo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "user-1", CalleeID: "other", Status: models.CallStatusActive}
	hub := newMockCallHub()
	svc := newTestCallService(repo, hub)

	_, err := svc.StartCall("user-1", StartCallInput{CalleeID: "someone"})
	if !errors.Is(err, ErrActiveCall) {
		t.Fatalf("expected ErrActiveCall, got %v", err)
	}
}

func TestAnswerCallWrongUserRejected(t *testing.T) {
	repo := newMockCallRepo()
	repo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "caller-1", CalleeID: "callee-1", Status: models.CallStatusRinging}
	hub := newMockCallHub()
	svc := newTestCallService(repo, hub)

	_, err := svc.AnswerCall("call-1", "intruder")
	if !errors.Is(err, ErrNoPermission) {
		t.Fatalf("expected ErrNoPermission, got %v", err)
	}
}

func TestAnswerCallAddsParticipantsAndNotifiesCaller(t *testing.T) {
	repo := newMockCallRepo()
	repo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "caller-1", CalleeID: "callee-1", Status: models.CallStatusRinging}
	hub := newMockCallHub()
	svc := newTestCallService(repo, hub)

	call, err := svc.AnswerCall("call-1", "callee-1")
	if err != nil {
		t.Fatalf("AnswerCall: %v", err)
	}
	if call.Status != models.CallStatusActive {
		t.Errorf("expected active, got %s", call.Status)
	}
	if len(repo.participants["call-1"]) != 2 {
		t.Errorf("expected 2 participants, got %d", len(repo.participants["call-1"]))
	}
	if hub.count("caller-1") != 1 {
		t.Errorf("expected accepted event to caller, got %d", hub.count("caller-1"))
	}
}

func TestRejectCallWrongUserRejected(t *testing.T) {
	repo := newMockCallRepo()
	repo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "caller-1", CalleeID: "callee-1", Status: models.CallStatusRinging}
	hub := newMockCallHub()
	svc := newTestCallService(repo, hub)

	_, err := svc.RejectCall("call-1", "intruder")
	if !errors.Is(err, ErrNoPermission) {
		t.Fatalf("expected ErrNoPermission, got %v", err)
	}
}

func TestEndCallNonParticipantRejected(t *testing.T) {
	repo := newMockCallRepo()
	repo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "caller-1", CalleeID: "callee-1", Status: models.CallStatusActive}
	hub := newMockCallHub()
	svc := newTestCallService(repo, hub)

	_, err := svc.EndCall("call-1", "intruder")
	if !errors.Is(err, ErrNoPermission) {
		t.Fatalf("expected ErrNoPermission, got %v", err)
	}
}

func TestEndCallByCallerNotifiesCallee(t *testing.T) {
	repo := newMockCallRepo()
	repo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "caller-1", CalleeID: "callee-1", Status: models.CallStatusActive}
	hub := newMockCallHub()
	svc := newTestCallService(repo, hub)

	call, err := svc.EndCall("call-1", "caller-1")
	if err != nil {
		t.Fatalf("EndCall: %v", err)
	}
	if call.Status != models.CallStatusEnded {
		t.Errorf("expected ended, got %s", call.Status)
	}
	if hub.count("callee-1") != 1 {
		t.Errorf("expected ended event to callee, got %d", hub.count("callee-1"))
	}
}

func TestSignalingRejectsNonParticipant(t *testing.T) {
	repo := newMockCallRepo()
	repo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "caller-1", CalleeID: "callee-1", Status: models.CallStatusActive}
	hub := newMockCallHub()
	svc := newTestCallService(repo, hub)

	err := svc.HandleSignaling("call-1", "intruder", "sdp", jsonMessage{"sdp": "x"})
	if !errors.Is(err, ErrNotParticipant) {
		t.Fatalf("expected ErrNotParticipant, got %v", err)
	}
}

func TestSignalingRelaysToOtherParty(t *testing.T) {
	repo := newMockCallRepo()
	repo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "caller-1", CalleeID: "callee-1", Status: models.CallStatusActive}
	hub := newMockCallHub()
	svc := newTestCallService(repo, hub)

	if err := svc.HandleSignaling("call-1", "caller-1", "sdp", jsonMessage{"sdp": "offer"}); err != nil {
		t.Fatalf("HandleSignaling: %v", err)
	}
	if hub.count("callee-1") != 1 {
		t.Errorf("expected relayed event to callee, got %d", hub.count("callee-1"))
	}
}

func TestGetHistoryFiltersByUser(t *testing.T) {
	repo := newMockCallRepo()
	repo.calls["call-1"] = &models.Call{ID: "call-1", CallerID: "a", CalleeID: "b", StartedAt: time.Now()}
	repo.calls["call-2"] = &models.Call{ID: "call-2", CallerID: "c", CalleeID: "d", StartedAt: time.Now()}
	hub := newMockCallHub()
	svc := newTestCallService(repo, hub)

	history, err := svc.GetHistory("a", 50, 0)
	if err != nil {
		t.Fatalf("GetHistory: %v", err)
	}
	if len(history) != 1 {
		t.Fatalf("expected 1 call for user a, got %d", len(history))
	}
}
