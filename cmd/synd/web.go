package main

import (
	"crypto/subtle"
	"embed"
	"html/template"
	"net/http"

	"github.com/benaskins/axon"
	synd "github.com/benaskins/axon-synd"
)

//go:embed all:templates
var templateFS embed.FS

var templates = template.Must(template.ParseFS(templateFS, "templates/*.html"))

type webHandler struct {
	store *synd.PostStore
}

func newWebHandler(store *synd.PostStore) *webHandler {
	return &webHandler{store: store}
}

// ShowDraft renders the review page for a draft post.
// Requires a valid approval token as a query parameter.
func (h *webHandler) ShowDraft(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token := r.URL.Query().Get("token")

	post := h.store.Get(id)
	if post == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if !validToken(token, post.ApprovalToken) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	data := map[string]any{
		"Post":  post,
		"Token": token,
	}
	templates.ExecuteTemplate(w, "draft.html", data)
}

// ReviseDraft updates the body of a draft post.
func (h *webHandler) ReviseDraft(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token := r.FormValue("token")
	body := r.FormValue("body")

	post := h.store.Get(id)
	if post == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if !validToken(token, post.ApprovalToken) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	h.store.Revise(r.Context(), id, body, post.Title, post.Abstract, post.Tags, axon.Username(r.Context()))

	http.Redirect(w, r, "/drafts/"+id+"?token="+token, http.StatusSeeOther)
}

// ApproveDraft marks a draft post as approved.
func (h *webHandler) ApproveDraft(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	token := r.FormValue("token")

	post := h.store.Get(id)
	if post == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}

	if !validToken(token, post.ApprovalToken) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}

	h.store.Approve(r.Context(), id, axon.Username(r.Context()))

	post = h.store.Get(id)
	data := map[string]any{
		"Post": post,
	}
	templates.ExecuteTemplate(w, "approved.html", data)
}

func validToken(provided, stored string) bool {
	if provided == "" || stored == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(stored)) == 1
}
