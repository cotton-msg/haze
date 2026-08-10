package service

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/cotton-msg/haze/backend/internal/models"
)

var (
	ErrCallNotFound   = errors.New("call not found")
	ErrActiveCall     = errors.New("user already has an active call")
	ErrSelfCall       = errors.New("cannot call yourself")
	ErrNotParticipant = errors.New("user is not a participant of this call")
)

const ringTimeout = 30 * time.Second

// CallHub рассылает события звонков. В проде это адаптер поверх общего
// WS-канала чата (Redis PubSub), чтобы у клиента был единый WebSocket.
type CallHub interface {
	SendToUser(userID string, data []byte)
}

// CallRepository — интерфейс звонков для сервисного слоя (тестируемость).
type CallRepository interface {
	Create(call *models.Call) error
	FindByID(id string) (*models.Call, error)
	UpdateStatus(id string, status models.CallStatus) error
	GetHistory(userID string, limit, offset int) ([]*models.Call, error)
	AddParticipant(callID, userID string) error
	GetActiveCall(userID string) (*models.Call, error)
}

type CallService struct {
	callRepo   CallRepository
	hub        CallHub
	iceServers []IceServer
}

// IceServer — запись RTCIceServer для фронта (STUN/TURN).
type IceServer struct {
	URLs       []string `json:"urls"`
	Username   string   `json:"username,omitempty"`
	Credential string   `json:"credential,omitempty"`
}

func NewCallService(callRepo CallRepository, hub CallHub) *CallService {
	return &CallService{callRepo: callRepo, hub: hub}
}

// SetIceServers задаёт список STUN/TURN серверов, выдаваемых фронту.
func (s *CallService) SetIceServers(servers []IceServer) {
	s.iceServers = servers
}

// IceServers возвращает конфигурацию WebRTC для клиента.
func (s *CallService) IceServers() []IceServer {
	if len(s.iceServers) == 0 {
		return []IceServer{{URLs: []string{"stun:stun.l.google.com:19302", "stun:stun1.l.google.com:19302"}}}
	}
	return s.iceServers
}

type StartCallInput struct {
	CalleeID string          `json:"callee_id"`
	Type     models.CallType `json:"type"`
}

func (s *CallService) StartCall(callerID string, input StartCallInput) (*models.Call, error) {
	if callerID == input.CalleeID {
		return nil, ErrSelfCall
	}

	active, err := s.callRepo.GetActiveCall(callerID)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to check active call: %w", err)
	}
	if active != nil {
		return nil, ErrActiveCall
	}

	call := &models.Call{
		CallerID:  callerID,
		CalleeID:  input.CalleeID,
		Type:      input.Type,
		Status:    models.CallStatusRinging,
		StartedAt: time.Now(),
	}

	if err := s.callRepo.Create(call); err != nil {
		return nil, fmt.Errorf("failed to create call: %w", err)
	}

	// Авто-завершение зависших ringing-звонков через 30 секунд.
	time.AfterFunc(ringTimeout, func() {
		current, err := s.callRepo.FindByID(call.ID)
		if err == nil && current.Status == models.CallStatusRinging {
			s.callRepo.UpdateStatus(call.ID, models.CallStatusMissed)
			event := map[string]interface{}{
				"type": "call_timeout",
				"payload": map[string]interface{}{
					"call_id": call.ID,
				},
			}
			if data, err := toJSON(event); err == nil {
				s.hub.SendToUser(call.CallerID, data)
				s.hub.SendToUser(call.CalleeID, data)
			}
		}
	})

	event := map[string]interface{}{
		"type": "call_incoming",
		"payload": map[string]interface{}{
			"call": call,
		},
	}
	if data, err := toJSON(event); err == nil {
		s.hub.SendToUser(input.CalleeID, data)
	}

	return call, nil
}

func (s *CallService) AnswerCall(callID, userID string) (*models.Call, error) {
	call, err := s.callRepo.FindByID(callID)
	if err != nil {
		return nil, ErrCallNotFound
	}

	if call.CalleeID != userID {
		return nil, ErrNoPermission
	}

	if call.Status != models.CallStatusRinging {
		return nil, fmt.Errorf("call is not ringing")
	}

	if err := s.callRepo.UpdateStatus(callID, models.CallStatusActive); err != nil {
		return nil, fmt.Errorf("failed to update call: %w", err)
	}

	if err := s.callRepo.AddParticipant(callID, userID); err != nil {
		return nil, fmt.Errorf("failed to add participant: %w", err)
	}
	if err := s.callRepo.AddParticipant(callID, call.CallerID); err != nil {
		return nil, fmt.Errorf("failed to add participant: %w", err)
	}
	call.Status = models.CallStatusActive

	event := map[string]interface{}{
		"type": "call_accepted",
		"payload": map[string]interface{}{
			"call": call,
		},
	}
	if data, err := toJSON(event); err == nil {
		s.hub.SendToUser(call.CallerID, data)
	}

	return call, nil
}

func (s *CallService) RejectCall(callID, userID string) (*models.Call, error) {
	call, err := s.callRepo.FindByID(callID)
	if err != nil {
		return nil, ErrCallNotFound
	}

	if call.CalleeID != userID {
		return nil, ErrNoPermission
	}

	if err := s.callRepo.UpdateStatus(callID, models.CallStatusRejected); err != nil {
		return nil, fmt.Errorf("failed to update call: %w", err)
	}
	call.Status = models.CallStatusRejected

	event := map[string]interface{}{
		"type": "call_rejected",
		"payload": map[string]interface{}{
			"call": call,
		},
	}
	if data, err := toJSON(event); err == nil {
		s.hub.SendToUser(call.CallerID, data)
	}

	return call, nil
}

func (s *CallService) EndCall(callID, userID string) (*models.Call, error) {
	call, err := s.callRepo.FindByID(callID)
	if err != nil {
		return nil, ErrCallNotFound
	}

	if call.CallerID != userID && call.CalleeID != userID {
		return nil, ErrNoPermission
	}

	if call.Status == models.CallStatusRinging {
		if err := s.callRepo.UpdateStatus(callID, models.CallStatusMissed); err != nil {
			return nil, fmt.Errorf("failed to update call: %w", err)
		}
		call.Status = models.CallStatusMissed
	} else {
		if err := s.callRepo.UpdateStatus(callID, models.CallStatusEnded); err != nil {
			return nil, fmt.Errorf("failed to update call: %w", err)
		}
		call.Status = models.CallStatusEnded
	}

	otherID := call.CallerID
	if otherID == userID {
		otherID = call.CalleeID
	}
	event := map[string]interface{}{
		"type": "call_ended",
		"payload": map[string]interface{}{
			"call": call,
		},
	}
	if data, err := toJSON(event); err == nil {
		s.hub.SendToUser(otherID, data)
	}

	return call, nil
}

func (s *CallService) GetHistory(userID string, limit, offset int) ([]*models.Call, error) {
	return s.callRepo.GetHistory(userID, limit, offset)
}

func (s *CallService) HandleSignaling(callID, userID string, msgType string, payload jsonMessage) error {
	call, err := s.callRepo.FindByID(callID)
	if err != nil {
		return ErrCallNotFound
	}

	if call.CallerID != userID && call.CalleeID != userID {
		return ErrNotParticipant
	}

	otherID := call.CallerID
	if otherID == userID {
		otherID = call.CalleeID
	}

	body := make(map[string]interface{}, len(payload)+1)
	for k, v := range payload {
		body[k] = v
	}
	body["call_id"] = callID

	event := map[string]interface{}{
		"type":    msgType,
		"payload": body,
	}
	if data, err := toJSON(event); err == nil {
		s.hub.SendToUser(otherID, data)
	}

	return nil
}

type jsonMessage map[string]interface{}
