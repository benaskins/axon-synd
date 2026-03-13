package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	gate "github.com/benaskins/axon-gate"
	synd "github.com/benaskins/axon-synd"
)

type apiHandler struct {
	store *synd.PostStore
}

func newAPIHandler(store *synd.PostStore) *apiHandler {
	return &apiHandler{store: store}
}

type createPostRequest struct {
	Kind     synd.PostKind `json:"kind"`
	Body     string        `json:"body"`
	Title    string        `json:"title,omitempty"`
	Abstract string        `json:"abstract,omitempty"`
	Tags     []string      `json:"tags,omitempty"`
}

type postResponse struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Status        string `json:"status"`
	ApprovalToken string `json:"approval_token,omitempty"`
}

// CreatePost handles POST /api/posts
func (h *apiHandler) CreatePost(w http.ResponseWriter, r *http.Request) {
	var req createPostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
		return
	}

	if req.Body == "" {
		http.Error(w, "body required", http.StatusBadRequest)
		return
	}
	if req.Kind == "" {
		req.Kind = synd.Short
	}

	token, err := gate.GenerateToken()
	if err != nil {
		http.Error(w, "token generation failed", http.StatusInternalServerError)
		return
	}

	var opts []synd.PostOption
	opts = append(opts, synd.WithApprovalToken(token))
	if req.Title != "" {
		opts = append(opts, synd.WithTitle(req.Title))
	}
	if req.Abstract != "" {
		opts = append(opts, synd.WithAbstract(req.Abstract))
	}
	if len(req.Tags) > 0 {
		opts = append(opts, synd.WithTags(req.Tags...))
	}

	post, err := h.store.Create(r.Context(), req.Kind, req.Body, opts...)
	if err != nil {
		http.Error(w, fmt.Sprintf("create: %v", err), http.StatusInternalServerError)
		return
	}

	// Send Signal notification if configured
	if signal, ok := signalClientFromEnv(); ok {
		reviewBase := os.Getenv("SYND_REVIEW_URL")
		if reviewBase == "" {
			reviewBase = "https://synd.studio.internal"
		}
		if err := sendDraftNotification(signal, post, reviewBase); err != nil {
			fmt.Fprintf(os.Stderr, "warning: signal notification failed: %v\n", err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(postResponse{
		ID:            post.ID,
		Kind:          string(post.Kind),
		Status:        string(post.Status),
		ApprovalToken: post.ApprovalToken,
	})
}

// ListDrafts handles GET /api/drafts
func (h *apiHandler) ListDrafts(w http.ResponseWriter, r *http.Request) {
	drafts := h.store.Projection().Drafts()

	type draftItem struct {
		ID   string `json:"id"`
		Kind string `json:"kind"`
		Body string `json:"body"`
	}

	items := make([]draftItem, len(drafts))
	for i, d := range drafts {
		body := d.Body
		if d.Kind == synd.Long && d.Title != "" {
			body = d.Title
		}
		if len(body) > 80 {
			body = body[:77] + "..."
		}
		items[i] = draftItem{ID: d.ID, Kind: string(d.Kind), Body: body}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(items)
}

// ApprovePost handles POST /api/drafts/{id}/approve
func (h *apiHandler) ApprovePost(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	post := h.store.Get(id)
	if post == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	if post.Status != synd.StatusDraft {
		http.Error(w, fmt.Sprintf("post is %s, not draft", post.Status), http.StatusConflict)
		return
	}

	approvedBy := "cli"
	if ab := r.URL.Query().Get("by"); ab != "" {
		approvedBy = ab
	}

	if err := h.store.Approve(r.Context(), id, approvedBy); err != nil {
		http.Error(w, fmt.Sprintf("approve: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "approved", "id": id})
}

// serviceURL returns the synd service URL, checking SYND_SERVICE_URL env var first.
func serviceURL() string {
	if u := os.Getenv("SYND_SERVICE_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return "http://synd.studio.internal"
}
