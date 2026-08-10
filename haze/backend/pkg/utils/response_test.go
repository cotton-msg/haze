package utils

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestErrorResponse(t *testing.T) {
	w := httptest.NewRecorder()
	ErrorResponse(w, http.StatusForbidden, "no access")

	if w.Code != http.StatusForbidden {
		t.Errorf("expected 403, got %d", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("expected application/json, got %q", ct)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["error"] != true {
		t.Errorf("expected error=true, got %v", resp["error"])
	}
	if resp["message"] != "no access" {
		t.Errorf("expected message 'no access', got %v", resp["message"])
	}
}

func TestSuccessResponse(t *testing.T) {
	w := httptest.NewRecorder()
	SuccessResponse(w, map[string]int{"count": 7})

	if w.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", w.Code)
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if resp["error"] != false {
		t.Errorf("expected error=false, got %v", resp["error"])
	}
	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected data object, got %v", resp["data"])
	}
	if data["count"].(float64) != 7 {
		t.Errorf("expected count 7, got %v", data["count"])
	}
}

func TestPaginationNormalize(t *testing.T) {
	p := Pagination{}
	p.Normalize()
	if p.Limit != 50 || p.Offset != 0 {
		t.Errorf("expected default 50/0, got %d/%d", p.Limit, p.Offset)
	}

	p = Pagination{Limit: 200, Offset: -3}
	p.Normalize()
	if p.Limit != 50 || p.Offset != 0 {
		t.Errorf("expected clamped 50/0, got %d/%d", p.Limit, p.Offset)
	}

	p = Pagination{Limit: 10, Offset: 20}
	p.Normalize()
	if p.Limit != 10 || p.Offset != 20 {
		t.Errorf("expected preserved 10/20, got %d/%d", p.Limit, p.Offset)
	}
}
