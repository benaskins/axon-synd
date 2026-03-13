package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/benaskins/axon"
	synd "github.com/benaskins/axon-synd"
)

// stubValidator is a test SessionValidator that always succeeds or fails.
type stubValidator struct {
	valid    bool
	username string
}

func (s *stubValidator) ValidateSession(token string) (*axon.SessionInfo, error) {
	if !s.valid {
		return nil, axon.ErrUnauthorized
	}
	return &axon.SessionInfo{
		Claims: map[string]any{
			"user_id":  "test-id",
			"username": s.username,
		},
	}, nil
}

func TestHealthEndpoint_NoAuthRequired(t *testing.T) {
	store, _ := newMemoryStore()
	sv := &stubValidator{valid: false}
	mux := buildMux(store, sv)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /health = %d, want 200", w.Code)
	}
}

func TestAPIEndpoint_RejectsUnauthenticated(t *testing.T) {
	store, _ := newMemoryStore()
	sv := &stubValidator{valid: false}
	mux := buildMux(store, sv)

	req := httptest.NewRequest("GET", "/api/drafts", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("GET /api/drafts without auth = %d, want 401", w.Code)
	}
}

func TestAPIEndpoint_AllowsAuthenticated(t *testing.T) {
	store, _ := newMemoryStore()
	sv := &stubValidator{valid: true, username: "ben"}
	mux := buildMux(store, sv)

	req := httptest.NewRequest("GET", "/api/drafts", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "valid-token"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/drafts with auth = %d, want 200", w.Code)
	}
}

func TestApprovePost_UsesAuthIdentity(t *testing.T) {
	store, _ := newMemoryStore()
	ctx := context.Background()
	post, _ := store.Create(ctx, synd.Short, "identity test", synd.WithApprovalToken("tok"))

	sv := &stubValidator{valid: true, username: "alice"}
	mux := buildMux(store, sv)

	req := httptest.NewRequest("POST", "/api/drafts/"+post.ID+"/approve", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "valid"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	got := store.Get(post.ID)
	if got.ApprovedBy != "alice" {
		t.Errorf("ApprovedBy = %q, want %q", got.ApprovedBy, "alice")
	}
}

func TestWebEndpoint_RedirectsUnauthenticated(t *testing.T) {
	store, _ := newMemoryStore()
	sv := &stubValidator{valid: false}
	mux := buildMux(store, sv)

	req := httptest.NewRequest("GET", "/drafts/some-id?token=abc", nil)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusFound {
		t.Fatalf("GET /drafts/some-id without auth = %d, want 302", w.Code)
	}
	loc := w.Header().Get("Location")
	if loc == "" {
		t.Fatal("expected Location header for redirect")
	}
}
