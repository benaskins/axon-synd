package synd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// MastodonConfig holds credentials for the Mastodon API.
type MastodonConfig struct {
	Instance    string // e.g. "https://aus.social"
	AccessToken string // OAuth access token
}

// MastodonClient posts to Mastodon via the REST API.
type MastodonClient struct {
	config     MastodonConfig
	httpClient *http.Client
	username   string
}

// NewMastodonClient creates a client with the given config.
func NewMastodonClient(config MastodonConfig) *MastodonClient {
	config.Instance = strings.TrimRight(config.Instance, "/")
	return &MastodonClient{
		config:     config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// VerifyCredentials checks the access token and retrieves the account username.
func (c *MastodonClient) VerifyCredentials(ctx context.Context) error {
	var account struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	}

	if err := c.get(ctx, "/api/v1/accounts/verify_credentials", &account); err != nil {
		return fmt.Errorf("verify credentials: %w", err)
	}

	c.username = account.Username
	return nil
}

// Post creates a status on Mastodon. Returns the status ID and URL.
func (c *MastodonClient) Post(ctx context.Context, text string) (id string, statusURL string, err error) {
	return c.createStatus(ctx, text, nil)
}

// PostWithLink creates a status with a URL appended.
func (c *MastodonClient) PostWithLink(ctx context.Context, text, linkURL string) (id string, statusURL string, err error) {
	fullText := text + "\n\n" + linkURL
	return c.createStatus(ctx, fullText, nil)
}

// PostWithImage uploads an image and creates a status with it attached.
func (c *MastodonClient) PostWithImage(ctx context.Context, text, imagePath, altText string) (id string, statusURL string, err error) {
	mediaID, err := c.uploadMedia(ctx, imagePath, altText)
	if err != nil {
		return "", "", fmt.Errorf("upload media: %w", err)
	}
	return c.createStatus(ctx, text, []string{mediaID})
}

func (c *MastodonClient) createStatus(ctx context.Context, text string, mediaIDs []string) (string, string, error) {
	form := url.Values{}
	form.Set("status", text)
	for _, id := range mediaIDs {
		form.Add("media_ids[]", id)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.Instance+"/api/v1/statuses", strings.NewReader(form.Encode()))
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Bearer "+c.config.AccessToken)

	var resp struct {
		ID  string `json:"id"`
		URL string `json:"url"`
	}

	if err := c.doRequest(req, &resp); err != nil {
		return "", "", fmt.Errorf("create status: %w", err)
	}

	return resp.ID, resp.URL, nil
}

func (c *MastodonClient) uploadMedia(ctx context.Context, path, description string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", path)
	if err != nil {
		return "", err
	}
	part.Write(data)

	if description != "" {
		writer.WriteField("description", description)
	}

	writer.Close()

	req, err := http.NewRequestWithContext(ctx, "POST", c.config.Instance+"/api/v2/media", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+c.config.AccessToken)

	var resp struct {
		ID string `json:"id"`
	}

	if err := c.doRequest(req, &resp); err != nil {
		return "", fmt.Errorf("upload media: %w", err)
	}

	return resp.ID, nil
}

func (c *MastodonClient) get(ctx context.Context, path string, result any) error {
	req, err := http.NewRequestWithContext(ctx, "GET", c.config.Instance+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.config.AccessToken)
	return c.doRequest(req, result)
}

func (c *MastodonClient) doRequest(req *http.Request, result any) error {
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s %s %d: %s", req.Method, req.URL.Path, resp.StatusCode, body)
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}

	return nil
}

// MastodonPostURL constructs the web URL for a Mastodon status.
func MastodonPostURL(instance, username, statusID string) string {
	return fmt.Sprintf("%s/@%s/%s", strings.TrimRight(instance, "/"), username, statusID)
}
