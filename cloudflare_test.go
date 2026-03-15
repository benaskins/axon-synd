package synd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"
)

func TestCfHash(t *testing.T) {
	data := []byte("<h1>hello</h1>")
	hash := cfHash(data, ".html")

	if len(hash) != 32 {
		t.Fatalf("expected 32 char hash, got %d: %s", len(hash), hash)
	}

	// Same input produces same hash.
	if cfHash(data, ".html") != hash {
		t.Error("hash not deterministic")
	}

	// Different extension produces different hash.
	if cfHash(data, ".css") == hash {
		t.Error("different extension should produce different hash")
	}

	// Different content produces different hash.
	if cfHash([]byte("<h1>world</h1>"), ".html") == hash {
		t.Error("different content should produce different hash")
	}
}

func TestWalkSite(t *testing.T) {
	dir := t.TempDir()

	// Create site structure.
	os.MkdirAll(filepath.Join(dir, "posts", "abc"), 0o755)
	os.MkdirAll(filepath.Join(dir, ".git", "objects"), 0o755)
	os.WriteFile(filepath.Join(dir, "index.html"), []byte("<h1>home</h1>"), 0o644)
	os.WriteFile(filepath.Join(dir, "style.css"), []byte("body{}"), 0o644)
	os.WriteFile(filepath.Join(dir, "posts", "abc", "index.html"), []byte("<h1>post</h1>"), 0o644)
	os.WriteFile(filepath.Join(dir, ".git", "objects", "pack"), []byte("git data"), 0o644)

	files, err := walkSite(dir)
	if err != nil {
		t.Fatalf("walkSite: %v", err)
	}

	paths := make(map[string]bool)
	for _, f := range files {
		paths[f.path] = true
	}

	if !paths["/index.html"] {
		t.Error("missing /index.html")
	}
	if !paths["/style.css"] {
		t.Error("missing /style.css")
	}
	if !paths["/posts/abc/index.html"] {
		t.Error("missing /posts/abc/index.html")
	}
	if paths["/.git/objects/pack"] {
		t.Error(".git directory should be skipped")
	}

	// Check content types.
	for _, f := range files {
		if f.path == "/index.html" && f.contentType != "text/html; charset=utf-8" {
			t.Errorf("index.html content type = %q, want text/html", f.contentType)
		}
		if f.path == "/style.css" && f.contentType != "text/css; charset=utf-8" {
			t.Errorf("style.css content type = %q, want text/css", f.contentType)
		}
	}
}

func TestCloudflareDeploy(t *testing.T) {
	// Create a site directory.
	siteDir := t.TempDir()
	os.WriteFile(filepath.Join(siteDir, "index.html"), []byte("<h1>hello</h1>"), 0o644)
	os.WriteFile(filepath.Join(siteDir, "style.css"), []byte("body{margin:0}"), 0o644)

	var mu sync.Mutex
	var (
		gotUploadToken  bool
		gotCheckMissing bool
		uploadedFiles   []string
		gotUpsert       bool
		gotDeployment   bool
		deployManifest  map[string]string
	)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		switch {
		case r.URL.Path == "/accounts/test-account/pages/projects/test-project/upload-token":
			gotUploadToken = true
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result":  map[string]string{"jwt": "test-jwt"},
			})

		case r.URL.Path == "/pages/assets/check-missing":
			gotCheckMissing = true
			body, _ := io.ReadAll(r.Body)
			var req struct {
				Hashes []string `json:"hashes"`
			}
			json.Unmarshal(body, &req)
			// Return all hashes as missing.
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result":  req.Hashes,
			})

		case r.URL.Path == "/pages/assets/upload":
			body, _ := io.ReadAll(r.Body)
			var entries []struct {
				Key string `json:"key"`
			}
			json.Unmarshal(body, &entries)
			for _, e := range entries {
				uploadedFiles = append(uploadedFiles, e.Key)
			}
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result":  nil,
			})

		case r.URL.Path == "/pages/assets/upsert-hashes":
			gotUpsert = true
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result":  nil,
			})

		case r.URL.Path == "/accounts/test-account/pages/projects/test-project/deployments":
			gotDeployment = true
			// Parse multipart to get manifest.
			r.ParseMultipartForm(10 << 20)
			if m := r.FormValue("manifest"); m != "" {
				json.Unmarshal([]byte(m), &deployManifest)
			}
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result":  map[string]string{"url": "https://abc123.test-project.pages.dev"},
			})

		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			http.Error(w, "not found", 404)
		}
	}))
	defer srv.Close()

	// Override the API base URL.
	origAPI := cloudflareAPI
	defer func() { setCloudflareAPI(origAPI) }()
	setCloudflareAPI(srv.URL)

	cfg := CloudflareConfig{
		AccountID:   "test-account",
		APIToken:    "test-token",
		ProjectName: "test-project",
	}

	if err := CloudflareDeploy(cfg, siteDir); err != nil {
		t.Fatalf("CloudflareDeploy: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()

	if !gotUploadToken {
		t.Error("expected upload token request")
	}
	if !gotCheckMissing {
		t.Error("expected check-missing request")
	}
	if len(uploadedFiles) != 2 {
		t.Errorf("expected 2 uploaded files, got %d", len(uploadedFiles))
	}
	if !gotUpsert {
		t.Error("expected upsert-hashes request")
	}
	if !gotDeployment {
		t.Error("expected deployment request")
	}
	if len(deployManifest) != 2 {
		t.Errorf("expected 2 entries in manifest, got %d", len(deployManifest))
	}
	if _, ok := deployManifest["/index.html"]; !ok {
		t.Error("manifest missing /index.html")
	}
	if _, ok := deployManifest["/style.css"]; !ok {
		t.Error("manifest missing /style.css")
	}
}

func TestCloudflareDeploy_NothingMissing(t *testing.T) {
	siteDir := t.TempDir()
	os.WriteFile(filepath.Join(siteDir, "index.html"), []byte("<h1>cached</h1>"), 0o644)

	uploaded := false

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/accounts/test-account/pages/projects/test-project/upload-token":
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result":  map[string]string{"jwt": "test-jwt"},
			})

		case r.URL.Path == "/pages/assets/check-missing":
			// Return empty — all files already cached.
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result":  []string{},
			})

		case r.URL.Path == "/pages/assets/upload":
			uploaded = true
			json.NewEncoder(w).Encode(map[string]any{"success": true, "result": nil})

		case r.URL.Path == "/accounts/test-account/pages/projects/test-project/deployments":
			json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"result":  map[string]string{"url": "https://cached.pages.dev"},
			})

		default:
			http.Error(w, "not found", 404)
		}
	}))
	defer srv.Close()

	origAPI := cloudflareAPI
	defer func() { setCloudflareAPI(origAPI) }()
	setCloudflareAPI(srv.URL)

	cfg := CloudflareConfig{
		AccountID:   "test-account",
		APIToken:    "test-token",
		ProjectName: "test-project",
	}

	if err := CloudflareDeploy(cfg, siteDir); err != nil {
		t.Fatalf("CloudflareDeploy: %v", err)
	}

	if uploaded {
		t.Error("should not upload when nothing is missing")
	}
}
