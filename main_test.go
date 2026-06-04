package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func newTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	return buildRouter()
}

func TestRoute_Convert_Compat(t *testing.T) {
	r := newTestRouter()
	body, _ := json.Marshal(map[string]string{"amount": "1234.56"})
	req := httptest.NewRequest("POST", "/api/convert", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["chinese"] != "壹仟贰佰叁拾肆圆伍角陆分" {
		t.Errorf("got %q", resp["chinese"])
	}
}

func TestRoute_Reverse(t *testing.T) {
	r := newTestRouter()
	body, _ := json.Marshal(map[string]string{"chinese": "壹仟贰佰叁拾肆圆伍角陆分"})
	req := httptest.NewRequest("POST", "/api/convert/reverse", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["amount"] != "1234.56" {
		t.Errorf("got %q", resp["amount"])
	}
}

func TestRoute_Verify_Match(t *testing.T) {
	r := newTestRouter()
	body, _ := json.Marshal(map[string]string{
		"amount":  "1234.56",
		"chinese": "壹仟贰佰叁拾肆圆伍角陆分",
	})
	req := httptest.NewRequest("POST", "/api/convert/verify", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 200 {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["match"] != true {
		t.Errorf("expected match=true, got %+v", resp)
	}
}

func TestRoute_Batch_OverLimit(t *testing.T) {
	r := newTestRouter()
	amounts := make([]string, 201)
	for i := range amounts {
		amounts[i] = "1"
	}
	body, _ := json.Marshal(map[string]any{"amounts": amounts})
	req := httptest.NewRequest("POST", "/api/convert/batch", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 413 {
		t.Fatalf("status %d, body %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "batch_too_large" {
		t.Errorf("got error %v", resp["error"])
	}
}

func TestRoute_InvalidFormat(t *testing.T) {
	r := newTestRouter()
	body := strings.NewReader(`{"amount":"abc"}`)
	req := httptest.NewRequest("POST", "/api/convert", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != 400 {
		t.Fatalf("status %d", w.Code)
	}
	var resp map[string]any
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["error"] != "invalid_format" {
		t.Errorf("got error %v", resp["error"])
	}
}

func TestRoute_OpenAPI_Served(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest("GET", "/openapi.json", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status %d", w.Code)
	}
	if !strings.Contains(w.Header().Get("Content-Type"), "json") {
		t.Errorf("expected JSON content-type, got %q", w.Header().Get("Content-Type"))
	}
}
