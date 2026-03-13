package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

func TestCreatePost_DoesNotLeakApprovalToken(t *testing.T) {
	store, _ := newMemoryStore()
	sv := &stubValidator{valid: true, username: "ben"}
	mux := buildMux(store, sv)

	body := `{"kind":"short","body":"test post"}`
	req := httptest.NewRequest("POST", "/api/posts", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: "valid"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201; body: %s", w.Code, w.Body.String())
	}

	var resp map[string]any
	json.NewDecoder(w.Body).Decode(&resp)

	if _, ok := resp["approval_token"]; ok {
		t.Error("response should not contain approval_token")
	}
}

func TestAPIErrors_DoNotLeakInternals(t *testing.T) {
	store, _ := newMemoryStore()
	ctx := context.Background()
	post, _ := store.Create(ctx, synd.Short, "already approved", synd.WithApprovalToken("tok"))
	store.Approve(ctx, post.ID, "test")

	sv := &stubValidator{valid: true, username: "ben"}
	mux := buildMux(store, sv)

	// Try to approve an already-approved post — the conflict message is fine,
	// but internal errors (create/approve failures) should not leak stack details.
	// We'll test the create path with an empty body to trigger a known error path.
	req := httptest.NewRequest("POST", "/api/posts", strings.NewReader(`{"body":""}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: "valid"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	// "body required" is a validation error, not an internal error — that's fine.
	// The real check: internal server error messages should be generic.
	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", w.Code)
	}
	if strings.Contains(w.Body.String(), "fmt.") || strings.Contains(w.Body.String(), "runtime.") {
		t.Error("error response should not contain Go internals")
	}
}

func TestRevisePost_UpdatesPublishedPost(t *testing.T) {
	store, _ := newMemoryStore()
	ctx := context.Background()
	post, _ := store.Create(ctx, synd.Short, "original text", synd.WithApprovalToken("tok"))
	store.Approve(ctx, post.ID, "ben")
	store.Publish(ctx, post.ID, "https://example.com/posts/"+post.ID)

	sv := &stubValidator{valid: true, username: "ben"}
	mux := buildMux(store, sv)

	body := `{"body":"updated text","title":"New Title"}`
	req := httptest.NewRequest("PUT", "/api/posts/"+post.ID, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: "valid"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	got := store.Get(post.ID)
	if got.Body != "updated text" {
		t.Errorf("Body = %q, want %q", got.Body, "updated text")
	}
	if got.Title != "New Title" {
		t.Errorf("Title = %q, want %q", got.Title, "New Title")
	}
}

func TestRevisePost_NotFound(t *testing.T) {
	store, _ := newMemoryStore()
	sv := &stubValidator{valid: true, username: "ben"}
	mux := buildMux(store, sv)

	body := `{"body":"updated"}`
	req := httptest.NewRequest("PUT", "/api/posts/nonexistent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(&http.Cookie{Name: "session", Value: "valid"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestRebuildSite_Endpoint(t *testing.T) {
	store, _ := newMemoryStore()
	sv := &stubValidator{valid: true, username: "ben"}

	rebuilt := false
	rebuildFn := func() error {
		rebuilt = true
		return nil
	}
	mux := buildMux(store, sv, withRebuild(rebuildFn))

	req := httptest.NewRequest("POST", "/api/site/rebuild", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: "valid"})
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if !rebuilt {
		t.Error("rebuild function was not called")
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
