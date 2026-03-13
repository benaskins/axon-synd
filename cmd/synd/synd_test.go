package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	synd "github.com/benaskins/axon-synd"
)

func TestSyndicateBluesky_ShortPost(t *testing.T) {
	var capturedText string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.createSession":
			json.NewEncoder(w).Encode(map[string]string{
				"did": "did:plc:test", "accessJwt": "tok", "refreshJwt": "ref",
			})
		case "/xrpc/com.atproto.repo.createRecord":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			record := body["record"].(map[string]any)
			capturedText = record["text"].(string)
			json.NewEncoder(w).Encode(map[string]string{
				"uri": "at://did:plc:test/app.bsky.feed.post/abc",
				"cid": "bafytest",
			})
		}
	}))
	defer srv.Close()

	store, _ := newMemoryStore()
	ctx := context.Background()
	post, err := store.Create(ctx, synd.Short, "hello from test")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	store.Publish(ctx, post.ID, "https://example.com/posts/"+post.ID)

	err = syndicateToBluesky(ctx, store, post, "https://example.com", synd.BlueskyConfig{
		Handle:   "test.bsky.social",
		Password: "pass",
		PDS:      srv.URL,
	})
	if err != nil {
		t.Fatalf("syndicateToBluesky: %v", err)
	}

	if capturedText != "hello from test" {
		t.Errorf("posted text = %q, want %q", capturedText, "hello from test")
	}

	records := store.Projection().Syndications(post.ID)
	if len(records) != 1 {
		t.Fatalf("got %d syndication records, want 1", len(records))
	}
	if records[0].Platform != string(synd.Bluesky) {
		t.Errorf("platform = %q", records[0].Platform)
	}
}

func TestSyndicateBluesky_LongPost(t *testing.T) {
	var capturedText string
	var capturedFacets []any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.createSession":
			json.NewEncoder(w).Encode(map[string]string{
				"did": "did:plc:test", "accessJwt": "tok", "refreshJwt": "ref",
			})
		case "/xrpc/com.atproto.repo.createRecord":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			record := body["record"].(map[string]any)
			capturedText = record["text"].(string)
			if f, ok := record["facets"]; ok {
				capturedFacets = f.([]any)
			}
			json.NewEncoder(w).Encode(map[string]string{
				"uri": "at://did:plc:test/app.bsky.feed.post/def",
				"cid": "bafylong",
			})
		}
	}))
	defer srv.Close()

	store, _ := newMemoryStore()
	ctx := context.Background()
	post, _ := store.Create(ctx, synd.Long, "# Full Article\n\nLong content here.",
		synd.WithTitle("Full Article"),
		synd.WithAbstract("A short summary."),
	)
	store.Publish(ctx, post.ID, "https://example.com/posts/"+post.ID)

	err := syndicateToBluesky(ctx, store, post, "https://example.com", synd.BlueskyConfig{
		Handle: "test.bsky.social", Password: "pass", PDS: srv.URL,
	})
	if err != nil {
		t.Fatalf("syndicateToBluesky: %v", err)
	}

	// Should post abstract + link, not full body
	if capturedText == "" {
		t.Fatal("no text posted")
	}
	if capturedText == post.Body {
		t.Error("long post should not send full body to bluesky")
	}
	if len(capturedFacets) == 0 {
		t.Error("long post should include a link facet")
	}
}

func TestSyndicateBluesky_SkipsImportedPost(t *testing.T) {
	called := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.createSession":
			json.NewEncoder(w).Encode(map[string]string{
				"did": "did:plc:test", "accessJwt": "tok", "refreshJwt": "ref",
			})
		case "/xrpc/com.atproto.repo.createRecord":
			called = true
			json.NewEncoder(w).Encode(map[string]string{
				"uri": "at://did:plc:test/app.bsky.feed.post/abc",
				"cid": "bafytest",
			})
		}
	}))
	defer srv.Close()

	store, _ := newMemoryStore()
	ctx := context.Background()
	// Post imported from bluesky should not be re-syndicated
	post, _ := store.Create(ctx, synd.Short, "old bsky post", synd.WithImportedFrom("bluesky"))

	err := syndicateToBluesky(ctx, store, post, "https://example.com", synd.BlueskyConfig{
		Handle: "test.bsky.social", Password: "pass", PDS: srv.URL,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("should not post to bluesky for a post imported from bluesky")
	}
}

func TestRunPostWithSyndicate(t *testing.T) {
	var postedText string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/xrpc/com.atproto.server.createSession":
			json.NewEncoder(w).Encode(map[string]string{
				"did": "did:plc:test", "accessJwt": "tok", "refreshJwt": "ref",
			})
		case "/xrpc/com.atproto.repo.createRecord":
			var body map[string]any
			json.NewDecoder(r.Body).Decode(&body)
			record := body["record"].(map[string]any)
			postedText = record["text"].(string)
			json.NewEncoder(w).Encode(map[string]string{
				"uri": "at://did:plc:test/app.bsky.feed.post/syn1",
				"cid": "bafysyn",
			})
		}
	}))
	defer srv.Close()

	// Set up a git repo as the site dir
	siteDir := setupTestSiteRepo(t)

	t.Setenv("SYND_BLUESKY_HANDLE", "test.bsky.social")
	t.Setenv("SYND_BLUESKY_PASSWORD", "pass")
	t.Setenv("SYND_BLUESKY_PDS", srv.URL)

	rootCmd.SetArgs([]string{"post", "--syndicate", "--site-dir", siteDir, "--base-url", "https://example.com", "test syndication"})
	rootCmd.SetContext(context.Background())

	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("Execute: %v", err)
	}

	if postedText != "test syndication" {
		t.Errorf("bluesky text = %q, want %q", postedText, "test syndication")
	}
}

func setupTestSiteRepo(t *testing.T) string {
	t.Helper()
	remote := t.TempDir()
	synd.TestGit(t, remote, "init", "--bare")

	dir := t.TempDir()
	synd.TestGit(t, dir, "clone", remote, "site")
	siteDir := dir + "/site"

	os.WriteFile(siteDir+"/CNAME", []byte("test.example.com"), 0o644)
	synd.TestGit(t, siteDir, "add", "-A")
	synd.TestGit(t, siteDir, "commit", "-m", "initial")
	synd.TestGit(t, siteDir, "push", "-u", "origin", "main")
	return siteDir
}

func TestSyndicateBluesky_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"AuthenticationRequired"}`))
	}))
	defer srv.Close()

	store, _ := newMemoryStore()
	ctx := context.Background()
	post, _ := store.Create(ctx, synd.Short, "will fail")

	err := syndicateToBluesky(ctx, store, post, "https://example.com", synd.BlueskyConfig{
		Handle: "bad.bsky.social", Password: "wrong", PDS: srv.URL,
	})
	if err == nil {
		t.Fatal("expected auth error")
	}
}

func TestSyndicateMastodon_ShortPost(t *testing.T) {
	var capturedStatus string
	srv := mastodonTestServer(t, func(status string, _ []string) {
		capturedStatus = status
	})
	defer srv.Close()

	store, _ := newMemoryStore()
	ctx := context.Background()
	post, err := store.Create(ctx, synd.Short, "hello mastodon")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	store.Publish(ctx, post.ID, "https://example.com/posts/"+post.ID)

	err = syndicateToMastodon(ctx, store, post, "https://example.com", synd.MastodonConfig{
		Instance:    srv.URL,
		AccessToken: "test-token",
	})
	if err != nil {
		t.Fatalf("syndicateToMastodon: %v", err)
	}

	if capturedStatus != "hello mastodon" {
		t.Errorf("posted status = %q, want %q", capturedStatus, "hello mastodon")
	}

	records := store.Projection().Syndications(post.ID)
	if len(records) != 1 {
		t.Fatalf("got %d syndication records, want 1", len(records))
	}
	if records[0].Platform != string(synd.Mastodon) {
		t.Errorf("platform = %q", records[0].Platform)
	}
}

func TestSyndicateMastodon_LongPost(t *testing.T) {
	var capturedStatus string
	srv := mastodonTestServer(t, func(status string, _ []string) {
		capturedStatus = status
	})
	defer srv.Close()

	store, _ := newMemoryStore()
	ctx := context.Background()
	post, _ := store.Create(ctx, synd.Long, "# Full Article\n\nLong content here.",
		synd.WithTitle("Full Article"),
		synd.WithAbstract("A short summary."),
	)
	store.Publish(ctx, post.ID, "https://example.com/posts/"+post.ID)

	err := syndicateToMastodon(ctx, store, post, "https://example.com", synd.MastodonConfig{
		Instance:    srv.URL,
		AccessToken: "test-token",
	})
	if err != nil {
		t.Fatalf("syndicateToMastodon: %v", err)
	}

	if capturedStatus == "" {
		t.Fatal("no status posted")
	}
	if capturedStatus == post.Body {
		t.Error("long post should not send full body to mastodon")
	}
	// Should contain the link
	if !contains(capturedStatus, "https://example.com/posts/") {
		t.Error("long post should include a link to the canonical post")
	}
}

func TestSyndicateMastodon_SkipsImportedPost(t *testing.T) {
	called := false
	srv := mastodonTestServer(t, func(_ string, _ []string) {
		called = true
	})
	defer srv.Close()

	store, _ := newMemoryStore()
	ctx := context.Background()
	post, _ := store.Create(ctx, synd.Short, "old mastodon post", synd.WithImportedFrom("mastodon"))

	err := syndicateToMastodon(ctx, store, post, "https://example.com", synd.MastodonConfig{
		Instance:    srv.URL,
		AccessToken: "test-token",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("should not post to mastodon for a post imported from mastodon")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func mastodonTestServer(t *testing.T, onPost func(status string, mediaIDs []string)) *httptest.Server {
	t.Helper()
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v1/accounts/verify_credentials":
			json.NewEncoder(w).Encode(map[string]any{
				"id":       "109876",
				"username": "genlevel",
			})
		case "/api/v1/statuses":
			r.ParseForm()
			status := r.FormValue("status")
			mediaIDs := r.Form["media_ids[]"]
			if onPost != nil {
				onPost(status, mediaIDs)
			}
			json.NewEncoder(w).Encode(map[string]any{
				"id":  "12345",
				"url": srv.URL + "/@genlevel/12345",
			})
		case "/api/v2/media":
			json.NewEncoder(w).Encode(map[string]any{
				"id":   "media-99",
				"type": "image",
			})
		default:
			http.Error(w, "not found", 404)
		}
	}))
	return srv
}
