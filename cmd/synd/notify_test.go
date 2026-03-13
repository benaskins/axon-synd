package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	gate "github.com/benaskins/axon-gate"
	synd "github.com/benaskins/axon-synd"
)

func TestSendDraftNotification(t *testing.T) {
	var capturedMessage string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)
		capturedMessage = payload["message"].(string)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	signal := gate.NewSignalClient(srv.URL, "+61400000000")

	post := &synd.Post{
		ID:            "abc-123",
		Kind:          synd.Short,
		Body:          "hello from the generative plane",
		ApprovalToken: "test-token-xyz",
	}

	err := sendDraftNotification(signal, post, "https://synd.studio.internal")
	if err != nil {
		t.Fatalf("sendDraftNotification: %v", err)
	}

	if !strings.Contains(capturedMessage, "New draft for review") {
		t.Errorf("message missing header: %q", capturedMessage)
	}
	if !strings.Contains(capturedMessage, "hello from the generative plane") {
		t.Errorf("message missing body: %q", capturedMessage)
	}
	if !strings.Contains(capturedMessage, "https://synd.studio.internal/drafts/abc-123?token=test-token-xyz") {
		t.Errorf("message missing review URL: %q", capturedMessage)
	}
}

func TestSendDraftNotification_LongPost(t *testing.T) {
	var capturedMessage string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		json.NewDecoder(r.Body).Decode(&payload)
		capturedMessage = payload["message"].(string)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	signal := gate.NewSignalClient(srv.URL, "+61400000000")

	post := &synd.Post{
		ID:            "def-456",
		Kind:          synd.Long,
		Title:         "My Long Article",
		Body:          "# My Long Article\n\nFull content here...",
		ApprovalToken: "tok",
	}

	err := sendDraftNotification(signal, post, "https://synd.studio.internal")
	if err != nil {
		t.Fatalf("sendDraftNotification: %v", err)
	}

	// Should use title, not full body
	if !strings.Contains(capturedMessage, "My Long Article") {
		t.Errorf("message missing title: %q", capturedMessage)
	}
	if strings.Contains(capturedMessage, "Full content here") {
		t.Errorf("message should not contain full body: %q", capturedMessage)
	}
}

func TestGenerateApprovalToken(t *testing.T) {
	token, err := gate.GenerateToken()
	if err != nil {
		t.Fatalf("GenerateToken: %v", err)
	}
	if len(token) < 32 {
		t.Errorf("token too short: %d chars", len(token))
	}

	// Should be unique
	token2, _ := gate.GenerateToken()
	if token == token2 {
		t.Error("tokens should be unique")
	}
}
