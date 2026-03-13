package synd

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestBlueskyClient_Authenticate(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/xrpc/com.atproto.server.createSession" {
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", 404)
			return
		}

		var body struct {
			Identifier string `json:"identifier"`
			Password   string `json:"password"`
		}
		json.NewDecoder(r.Body).Decode(&body)

		if body.Identifier != "baskins.bsky.social" {
			t.Errorf("identifier = %q", body.Identifier)
		}

		json.NewEncoder(w).Encode(map[string]string{
			"did":        "did:plc:test123",
			"accessJwt":  "access-token",
			"refreshJwt": "refresh-token",
			"handle":     "baskins.bsky.social",
		})
	}))
	defer srv.Close()

	client := NewBlueskyClient(BlueskyConfig{
		Handle:   "baskins.bsky.social",
		Password: "test-app-pass",
		PDS:      srv.URL,
	})

	if err := client.Authenticate(context.Background()); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}

	if client.did != "did:plc:test123" {
		t.Errorf("did = %q, want did:plc:test123", client.did)
	}
	if client.accessJwt != "access-token" {
		t.Errorf("accessJwt = %q", client.accessJwt)
	}
}

func TestBlueskyClient_Post(t *testing.T) {
	srv := blueskyTestServer(t)
	defer srv.Close()

	client := authenticatedClient(t, srv)

	uri, cid, err := client.Post(context.Background(), "hello from synd")
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if uri != "at://did:plc:test123/app.bsky.feed.post/abc" {
		t.Errorf("uri = %q", uri)
	}
	if cid != "bafytest" {
		t.Errorf("cid = %q", cid)
	}
}

func TestBlueskyClient_PostWithLink(t *testing.T) {
	var capturedRecord map[string]any

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.createSession":
			json.NewEncoder(w).Encode(map[string]string{
				"did":        "did:plc:test123",
				"accessJwt":  "token",
				"refreshJwt": "refresh",
			})
		case "/xrpc/com.atproto.repo.createRecord":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			capturedRecord = body["record"].(map[string]any)
			json.NewEncoder(w).Encode(map[string]string{
				"uri": "at://did:plc:test123/app.bsky.feed.post/abc",
				"cid": "bafytest",
			})
		}
	}))
	defer srv.Close()

	client := authenticatedClient(t, srv)

	_, _, err := client.PostWithLink(context.Background(),
		"New article",
		"https://generativeplane.com/posts/123",
		"Read more",
	)
	if err != nil {
		t.Fatalf("PostWithLink: %v", err)
	}

	text, ok := capturedRecord["text"].(string)
	if !ok || text != "New article\n\nRead more" {
		t.Errorf("text = %q", text)
	}

	facets, ok := capturedRecord["facets"].([]any)
	if !ok || len(facets) != 1 {
		t.Fatalf("facets = %v", capturedRecord["facets"])
	}

	facet := facets[0].(map[string]any)
	index := facet["index"].(map[string]any)

	// "New article\n\n" = 13 bytes, "Read more" = 9 bytes
	if int(index["byteStart"].(float64)) != 13 {
		t.Errorf("byteStart = %v, want 13", index["byteStart"])
	}
	if int(index["byteEnd"].(float64)) != 22 {
		t.Errorf("byteEnd = %v, want 22", index["byteEnd"])
	}
}

func TestBlueskyClient_PostWithImage(t *testing.T) {
	var capturedRecord map[string]any
	var uploadedContentType string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.createSession":
			json.NewEncoder(w).Encode(map[string]string{
				"did":        "did:plc:test123",
				"accessJwt":  "token",
				"refreshJwt": "refresh",
			})
		case "/xrpc/com.atproto.repo.uploadBlob":
			uploadedContentType = r.Header.Get("Content-Type")
			json.NewEncoder(w).Encode(map[string]any{
				"blob": map[string]any{
					"$type":    "blob",
					"ref":      map[string]string{"$link": "bafkreiblob"},
					"mimeType": uploadedContentType,
					"size":     r.ContentLength,
				},
			})
		case "/xrpc/com.atproto.repo.createRecord":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			capturedRecord = body["record"].(map[string]any)
			json.NewEncoder(w).Encode(map[string]string{
				"uri": "at://did:plc:test123/app.bsky.feed.post/img1",
				"cid": "bafyimg",
			})
		}
	}))
	defer srv.Close()

	client := authenticatedClient(t, srv)

	// Create a temp image file
	tmpFile := t.TempDir() + "/test.png"
	os.WriteFile(tmpFile, []byte("fake-png-data"), 0o644)

	uri, _, err := client.PostWithImage(context.Background(), "studio session", tmpFile, "a photo")
	if err != nil {
		t.Fatalf("PostWithImage: %v", err)
	}

	if uri != "at://did:plc:test123/app.bsky.feed.post/img1" {
		t.Errorf("uri = %q", uri)
	}
	if uploadedContentType != "image/png" {
		t.Errorf("content-type = %q, want image/png", uploadedContentType)
	}

	embed, ok := capturedRecord["embed"].(map[string]any)
	if !ok {
		t.Fatal("missing embed in record")
	}
	if embed["$type"] != "app.bsky.embed.images" {
		t.Errorf("embed type = %q", embed["$type"])
	}

	images := embed["images"].([]any)
	if len(images) != 1 {
		t.Fatalf("got %d images, want 1", len(images))
	}
	img := images[0].(map[string]any)
	if img["alt"] != "a photo" {
		t.Errorf("alt = %q, want %q", img["alt"], "a photo")
	}
}

func TestBlueskyPostURL(t *testing.T) {
	tests := []struct {
		handle string
		atURI  string
		want   string
	}{
		{
			"baskins.bsky.social",
			"at://did:plc:abc123/app.bsky.feed.post/xyz789",
			"https://bsky.app/profile/baskins.bsky.social/post/xyz789",
		},
	}

	for _, tt := range tests {
		got := BlueskyPostURL(tt.handle, tt.atURI)
		if got != tt.want {
			t.Errorf("BlueskyPostURL(%q, %q) = %q, want %q", tt.handle, tt.atURI, got, tt.want)
		}
	}
}

func TestBlueskyClient_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"AuthenticationRequired","message":"Invalid identifier or password"}`))
	}))
	defer srv.Close()

	client := NewBlueskyClient(BlueskyConfig{
		Handle:   "bad.bsky.social",
		Password: "wrong",
		PDS:      srv.URL,
	})

	err := client.Authenticate(context.Background())
	if err == nil {
		t.Fatal("expected auth error")
	}
}

// Test helpers

func blueskyTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.createSession":
			json.NewEncoder(w).Encode(map[string]string{
				"did":        "did:plc:test123",
				"accessJwt":  "token",
				"refreshJwt": "refresh",
			})
		case "/xrpc/com.atproto.repo.createRecord":
			json.NewEncoder(w).Encode(map[string]string{
				"uri": "at://did:plc:test123/app.bsky.feed.post/abc",
				"cid": "bafytest",
			})
		case "/xrpc/com.atproto.repo.uploadBlob":
			json.NewEncoder(w).Encode(map[string]any{
				"blob": map[string]any{
					"$type":    "blob",
					"ref":      map[string]string{"$link": "bafkreiblob"},
					"mimeType": r.Header.Get("Content-Type"),
					"size":     r.ContentLength,
				},
			})
		default:
			http.Error(w, "not found", 404)
		}
	}))
}

func authenticatedClient(t *testing.T, srv *httptest.Server) *BlueskyClient {
	t.Helper()
	client := NewBlueskyClient(BlueskyConfig{
		Handle:   "baskins.bsky.social",
		Password: "test-pass",
		PDS:      srv.URL,
	})
	if err := client.Authenticate(context.Background()); err != nil {
		t.Fatalf("Authenticate: %v", err)
	}
	return client
}
