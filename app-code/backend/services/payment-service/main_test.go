package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealthHandler(t *testing.T) {
	t.Run("returns healthy service JSON", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		healthHandler(recorder, httptest.NewRequest(http.MethodGet, "/health", nil))

		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusOK)
		}
		if got := recorder.Header().Get("Content-Type"); got != "application/json" {
			t.Fatalf("Content-Type = %q, want application/json", got)
		}

		var response map[string]string
		if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if response["service"] != "payment-service" || response["status"] != "UP" {
			t.Fatalf("response = %#v, want payment-service UP", response)
		}
	})

	t.Run("rejects non-GET requests", func(t *testing.T) {
		recorder := httptest.NewRecorder()
		healthHandler(recorder, httptest.NewRequest(http.MethodPost, "/health", nil))

		if recorder.Code != http.StatusMethodNotAllowed {
			t.Fatalf("status = %d, want %d", recorder.Code, http.StatusMethodNotAllowed)
		}
	})
}
