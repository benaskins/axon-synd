package synd

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestMastodonClient_Post(t *testing.T) {
	var capturedStatus string

	srv := mastodonTestServer(t, func(status string, mediaIDs []string) {
		capturedStatus = status
	})
	defer srv.Close()

	client := NewMastodonClient(MastodonConfig{
		Instance:    srv.URL,
		AccessToken: "test-token",
	})

	id, url, err := client.Post(context.Background(), "hello from synd")
	if err != nil {
		t.Fatalf("Post: %v", err)
	}
	if id != "12345" {
		t.Errorf("id = %q, want 12345", id)
	}
	if url != srv.URL+"/@genlevel/12345" {
		t.Errorf("url = %q", url)
	}
	if capturedStatus != "hello from synd" {
		t.Errorf("status = %q, want %q", capturedStatus, "hello from synd")
	}
}

func TestMastodonClient_PostWithLink(t *testing.T) {
	var capturedStatus string

	srv := mastodonTestServer(t, func(status string, mediaIDs []string) {
		capturedStatus = status
	})
	defer srv.Close()

	client := NewMastodonClient(MastodonConfig{
		Instance:    srv.URL,
		AccessToken: "test-token",
	})

	_, _, err := client.PostWithLink(context.Background(),
		"New article",
		"https://generativeplane.com/posts/123",
	)
	if err != nil {
		t.Fatalf("PostWithLink: %v", err)
	}

	want := "New article\n\nhttps://generativeplane.com/posts/123"
	if capturedStatus != want {
		t.Errorf("status = %q, want %q", capturedStatus, want)
	}
}

func TestMastodonClient_PostWithImage(t *testing.T) {
	var capturedStatus string
	var capturedMediaIDs []string
	var uploadedDescription string

	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/v2/media":
			if r.Header.Get("Authorization") != "Bearer test-token" {
				t.Errorf("missing auth header")
			}

			mediaType, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if mediaType != "multipart/form-data" {
				t.Errorf("content-type = %q", mediaType)
			}
			reader := multipart.NewReader(r.Body, params["boundary"])
			for {
				part, err := reader.NextPart()
				if err != nil {
					break
				}
				if part.FormName() == "description" {
					data, _ := io.ReadAll(part)
					uploadedDescription = string(data)
				}
			}

			json.NewEncoder(w).Encode(map[string]any{
				"id":   "media-99",
				"type": "image",
			})

		case "/api/v1/statuses":
			r.ParseForm()
			capturedStatus = r.FormValue("status")
			capturedMediaIDs = r.Form["media_ids[]"]

			json.NewEncoder(w).Encode(map[string]any{
				"id":  "12345",
				"url": srv.URL + "/@genlevel/12345",
			})
		}
	}))
	defer srv.Close()

	client := NewMastodonClient(MastodonConfig{
		Instance:    srv.URL,
		AccessToken: "test-token",
	})

	tmpFile := t.TempDir() + "/test.png"
	os.WriteFile(tmpFile, []byte("fake-png-data"), 0o644)

	id, _, err := client.PostWithImage(context.Background(), "studio session", tmpFile, "a photo")
	if err != nil {
		t.Fatalf("PostWithImage: %v", err)
	}

	if id != "12345" {
		t.Errorf("id = %q", id)
	}
	if capturedStatus != "studio session" {
		t.Errorf("status = %q", capturedStatus)
	}
	if len(capturedMediaIDs) != 1 || capturedMediaIDs[0] != "media-99" {
		t.Errorf("media_ids = %v", capturedMediaIDs)
	}
	if uploadedDescription != "a photo" {
		t.Errorf("description = %q, want %q", uploadedDescription, "a photo")
	}
}

func TestMastodonClient_AuthFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"The access token is invalid"}`))
	}))
	defer srv.Close()

	client := NewMastodonClient(MastodonConfig{
		Instance:    srv.URL,
		AccessToken: "bad-token",
	})

	_, _, err := client.Post(context.Background(), "should fail")
	if err == nil {
		t.Fatal("expected auth error")
	}
}

func TestMastodonClient_VerifyCredentials(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/accounts/verify_credentials" {
			http.Error(w, "not found", 404)
			return
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{
			"id":       "109876",
			"username": "genlevel",
			"acct":     "genlevel",
		})
	}))
	defer srv.Close()

	client := NewMastodonClient(MastodonConfig{
		Instance:    srv.URL,
		AccessToken: "test-token",
	})

	if err := client.VerifyCredentials(context.Background()); err != nil {
		t.Fatalf("VerifyCredentials: %v", err)
	}
	if client.username != "genlevel" {
		t.Errorf("username = %q, want genlevel", client.username)
	}
}

func TestMastodonPostURL(t *testing.T) {
	got := MastodonPostURL("https://aus.social", "genlevel", "12345")
	want := "https://aus.social/@genlevel/12345"
	if got != want {
		t.Errorf("MastodonPostURL = %q, want %q", got, want)
	}
}

// Test helpers

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
