package vllm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestClient(server *httptest.Server) *Client {
	// Strip the scheme so NewClient can prepend "http://".
	addr := strings.TrimPrefix(server.URL, "http://")
	return &Client{baseURL: server.URL, http: server.Client()}
}

// Test 9: a well-formed 200 response returns the assistant message content.
func TestSummarize_success(t *testing.T) {
	const wantSummary = "The speaker discussed quarterly results."

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %s, want /v1/chat/completions", r.URL.Path)
		}

		resp := chatResponse{
			Choices: []struct {
				Message message `json:"message"`
			}{
				{Message: message{Role: "assistant", Content: wantSummary}},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}))
	defer srv.Close()

	got, err := newTestClient(srv).Summarize(context.Background(), "some transcript")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != wantSummary {
		t.Errorf("summary = %q, want %q", got, wantSummary)
	}
}

// Test 10: a non-200 response is returned as an error containing the status code.
func TestSummarize_serverError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "model not loaded", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := newTestClient(srv).Summarize(context.Background(), "some transcript")
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("error should mention status 500, got: %v", err)
	}
}
