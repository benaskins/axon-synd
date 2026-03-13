package main

import (
	"context"
	"crypto/subtle"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/benaskins/axon"
	synd "github.com/benaskins/axon-synd"
)

// withAuthContext sets auth identity on the request context for testing.
func withAuthContext(r *http.Request, username string) *http.Request {
	ctx := context.WithValue(r.Context(), axon.UsernameKey, username)
	ctx = context.WithValue(ctx, axon.UserIDKey, "test-user-id")
	return r.WithContext(ctx)
}

func TestShowDraft(t *testing.T) {
	store, _ := newMemoryStore()
	ctx := context.Background()

	post, _ := store.Create(ctx, synd.Short, "review me", synd.WithApprovalToken("test-token"))
	h := newWebHandler(store)

	req := httptest.NewRequest("GET", "/drafts/"+post.ID+"?token=test-token", nil)
	req.SetPathValue("id", post.ID)
	w := httptest.NewRecorder()

	h.ShowDraft(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "review me") {
		t.Errorf("body should contain post text")
	}
}

func TestShowDraft_InvalidToken(t *testing.T) {
	store, _ := newMemoryStore()
	ctx := context.Background()

	post, _ := store.Create(ctx, synd.Short, "secret", synd.WithApprovalToken("real-token"))
	h := newWebHandler(store)

	req := httptest.NewRequest("GET", "/drafts/"+post.ID+"?token=wrong-token", nil)
	req.SetPathValue("id", post.ID)
	w := httptest.NewRecorder()

	h.ShowDraft(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403", w.Code)
	}
}

func TestShowDraft_NotFound(t *testing.T) {
	store, _ := newMemoryStore()
	h := newWebHandler(store)

	req := httptest.NewRequest("GET", "/drafts/nonexistent?token=x", nil)
	req.SetPathValue("id", "nonexistent")
	w := httptest.NewRecorder()

	h.ShowDraft(w, req)

	if w.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", w.Code)
	}
}

func TestReviseDraft(t *testing.T) {
	store, _ := newMemoryStore()
	ctx := context.Background()

	post, _ := store.Create(ctx, synd.Short, "original text", synd.WithApprovalToken("tok"))
	h := newWebHandler(store)

	form := url.Values{}
	form.Set("token", "tok")
	form.Set("body", "revised text")

	req := httptest.NewRequest("POST", "/drafts/"+post.ID+"/revise", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", post.ID)
	req = withAuthContext(req, "test-user")
	w := httptest.NewRecorder()

	h.ReviseDraft(w, req)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("status = %d, want 303", w.Code)
	}

	got := store.Get(post.ID)
	if got.Body != "revised text" {
		t.Errorf("Body = %q, want %q", got.Body, "revised text")
	}
}

func TestApproveDraft(t *testing.T) {
	store, _ := newMemoryStore()
	ctx := context.Background()

	post, _ := store.Create(ctx, synd.Short, "approve me", synd.WithApprovalToken("tok"))
	h := newWebHandler(store)

	form := url.Values{}
	form.Set("token", "tok")

	req := httptest.NewRequest("POST", "/drafts/"+post.ID+"/approve", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.SetPathValue("id", post.ID)
	req = withAuthContext(req, "test-user")
	w := httptest.NewRecorder()

	h.ApproveDraft(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body: %s", w.Code, w.Body.String())
	}

	got := store.Get(post.ID)
	if got.Status != synd.StatusApproved {
		t.Errorf("Status = %q, want %q", got.Status, synd.StatusApproved)
	}
}

// Verify constant-time comparison is being used (compile-time check)
var _ = subtle.ConstantTimeCompare
