package synd

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"mime"
	"net/http"
	"net/textproto"
	"os"
	"path/filepath"
	"strings"

	"github.com/zeebo/blake3"
)

var cloudflareAPI = "https://api.cloudflare.com/client/v4"

const (
	// Cloudflare Pages limits.
	maxUploadBatchSize  = 40 * 1024 * 1024 // 40 MiB per request
	maxUploadBatchFiles = 2000
)

// setCloudflareAPI overrides the API base URL. For testing only.
func setCloudflareAPI(url string) { cloudflareAPI = url }

// CloudflareConfig holds credentials for Cloudflare Pages direct upload.
type CloudflareConfig struct {
	AccountID   string
	APIToken    string
	ProjectName string
}

// CloudflareDeploy uploads the contents of siteDir to a Cloudflare Pages
// project using the Direct Upload API. It hashes all files with BLAKE3,
// checks which are missing, uploads them, and creates a deployment.
func CloudflareDeploy(cfg CloudflareConfig, siteDir string) error {
	files, err := walkSite(siteDir)
	if err != nil {
		return fmt.Errorf("walk site: %w", err)
	}

	if len(files) == 0 {
		return fmt.Errorf("no files found in %s", siteDir)
	}

	manifest := make(map[string]string, len(files))
	byHash := make(map[string]*siteFile, len(files))
	hashes := make([]string, 0, len(files))
	for _, f := range files {
		manifest[f.path] = f.hash
		byHash[f.hash] = f
		hashes = append(hashes, f.hash)
	}

	client := &http.Client{}

	// Step 1: Get upload JWT.
	jwt, err := cfUploadToken(client, cfg)
	if err != nil {
		return fmt.Errorf("upload token: %w", err)
	}

	// Step 2: Check which files are missing.
	missing, err := cfCheckMissing(client, jwt, hashes)
	if err != nil {
		return fmt.Errorf("check missing: %w", err)
	}

	slog.Info("cloudflare deploy", "total_files", len(files), "to_upload", len(missing))

	// Step 3: Upload missing files in batches.
	if len(missing) > 0 {
		var toUpload []*siteFile
		for _, h := range missing {
			if f, ok := byHash[h]; ok {
				toUpload = append(toUpload, f)
			}
		}
		if err := cfUploadFiles(client, jwt, toUpload, siteDir); err != nil {
			return fmt.Errorf("upload files: %w", err)
		}

		// Step 4: Upsert hashes.
		if err := cfUpsertHashes(client, jwt, missing); err != nil {
			return fmt.Errorf("upsert hashes: %w", err)
		}
	}

	// Step 5: Create deployment with manifest.
	url, err := cfCreateDeployment(client, cfg, manifest)
	if err != nil {
		return fmt.Errorf("create deployment: %w", err)
	}

	slog.Info("cloudflare deployed", "url", url)
	return nil
}

type siteFile struct {
	path        string // URL path, e.g. "/index.html"
	diskPath    string // relative path on disk
	hash        string
	contentType string
}

// walkSite walks a directory and returns hashed file entries.
func walkSite(dir string) ([]*siteFile, error) {
	var files []*siteFile
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip .git directory.
			if d.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}

		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		hash := cfHash(data, filepath.Ext(rel))

		urlPath := "/" + filepath.ToSlash(rel)
		ct := contentTypeForPath(rel)

		files = append(files, &siteFile{
			path:        urlPath,
			diskPath:    rel,
			hash:        hash,
			contentType: ct,
		})
		return nil
	})
	return files, err
}

// cfHash computes the Cloudflare Pages file hash:
// BLAKE3(base64(content) + extension)[:32] as hex.
func cfHash(data []byte, ext string) string {
	ext = strings.TrimPrefix(ext, ".")
	input := base64.StdEncoding.EncodeToString(data) + ext
	h := blake3.Sum256([]byte(input))
	return fmt.Sprintf("%x", h)[:32]
}

func contentTypeForPath(path string) string {
	ext := filepath.Ext(path)
	ct := mime.TypeByExtension(ext)
	if ct == "" {
		ct = "application/octet-stream"
	}
	return ct
}

// cfUploadToken gets a short-lived JWT for uploading assets.
func cfUploadToken(client *http.Client, cfg CloudflareConfig) (string, error) {
	url := fmt.Sprintf("%s/accounts/%s/pages/projects/%s/upload-token",
		cloudflareAPI, cfg.AccountID, cfg.ProjectName)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIToken)

	var result struct {
		JWT string `json:"jwt"`
	}
	if err := cfDo(client, req, &result); err != nil {
		return "", err
	}
	return result.JWT, nil
}

// cfCheckMissing returns the subset of hashes not already stored.
func cfCheckMissing(client *http.Client, jwt string, hashes []string) ([]string, error) {
	body, err := json.Marshal(map[string][]string{"hashes": hashes})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", cloudflareAPI+"/pages/assets/check-missing", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")

	var result []string
	if err := cfDo(client, req, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// cfUploadFiles uploads files in batches.
func cfUploadFiles(client *http.Client, jwt string, files []*siteFile, siteDir string) error {
	type uploadEntry struct {
		Key      string            `json:"key"`
		Value    string            `json:"value"`
		Metadata map[string]string `json:"metadata"`
		Base64   bool              `json:"base64"`
	}

	var batch []uploadEntry
	var batchSize int

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		body, err := json.Marshal(batch)
		if err != nil {
			return err
		}

		req, err := http.NewRequest("POST", cloudflareAPI+"/pages/assets/upload", bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+jwt)
		req.Header.Set("Content-Type", "application/json")

		if err := cfDo(client, req, nil); err != nil {
			return err
		}

		batch = batch[:0]
		batchSize = 0
		return nil
	}

	for _, f := range files {
		data, err := os.ReadFile(filepath.Join(siteDir, f.diskPath))
		if err != nil {
			return fmt.Errorf("read %s: %w", f.diskPath, err)
		}

		encoded := base64.StdEncoding.EncodeToString(data)
		entrySize := len(encoded) + len(f.hash) + 100 // approximate JSON overhead

		if len(batch) > 0 && (len(batch) >= maxUploadBatchFiles || batchSize+entrySize > maxUploadBatchSize) {
			if err := flush(); err != nil {
				return err
			}
		}

		batch = append(batch, uploadEntry{
			Key:      f.hash,
			Value:    encoded,
			Metadata: map[string]string{"contentType": f.contentType},
			Base64:   true,
		})
		batchSize += entrySize
	}

	return flush()
}

// cfUpsertHashes registers uploaded file hashes.
func cfUpsertHashes(client *http.Client, jwt string, hashes []string) error {
	body, err := json.Marshal(map[string][]string{"hashes": hashes})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", cloudflareAPI+"/pages/assets/upsert-hashes", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+jwt)
	req.Header.Set("Content-Type", "application/json")

	return cfDo(client, req, nil)
}

// cfCreateDeployment creates a deployment with a file manifest.
func cfCreateDeployment(client *http.Client, cfg CloudflareConfig, manifest map[string]string) (string, error) {
	manifestJSON, err := json.Marshal(manifest)
	if err != nil {
		return "", err
	}

	// Build multipart body.
	var body bytes.Buffer
	boundary := "----CloudflarePagesDeploy"
	w := newMultipartWriter(&body, boundary)
	w.writeField("manifest", string(manifestJSON))
	w.writeField("branch", "main")
	w.close()

	url := fmt.Sprintf("%s/accounts/%s/pages/projects/%s/deployments",
		cloudflareAPI, cfg.AccountID, cfg.ProjectName)

	req, err := http.NewRequest("POST", url, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+cfg.APIToken)
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)

	var result struct {
		URL string `json:"url"`
	}
	if err := cfDo(client, req, &result); err != nil {
		return "", err
	}
	return result.URL, nil
}

// cfDo executes a request and decodes the Cloudflare API response.
// If dest is nil, the response body is discarded.
func cfDo(client *http.Client, req *http.Request, dest any) error {
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("cloudflare API %s %s: %d: %s", req.Method, req.URL.Path, resp.StatusCode, body)
	}

	if dest == nil {
		return nil
	}

	// Cloudflare wraps results in {"success": true, "result": ...}.
	var envelope struct {
		Success bool            `json:"success"`
		Result  json.RawMessage `json:"result"`
		Errors  json.RawMessage `json:"errors"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}
	if !envelope.Success {
		return fmt.Errorf("cloudflare API error: %s", envelope.Errors)
	}

	return json.Unmarshal(envelope.Result, dest)
}

// Simple multipart writer — avoids pulling in mime/multipart for form fields only.
type multipartWriter struct {
	w        io.Writer
	boundary string
}

func newMultipartWriter(w io.Writer, boundary string) *multipartWriter {
	return &multipartWriter{w: w, boundary: boundary}
}

func (m *multipartWriter) writeField(name, value string) {
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="%s"`, name))
	fmt.Fprintf(m.w, "--%s\r\n", m.boundary)
	for k, v := range h {
		fmt.Fprintf(m.w, "%s: %s\r\n", k, v[0])
	}
	fmt.Fprintf(m.w, "\r\n%s\r\n", value)
}

func (m *multipartWriter) close() {
	fmt.Fprintf(m.w, "--%s--\r\n", m.boundary)
}
