package synd

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const defaultPDS = "https://bsky.social"

// BlueskyConfig holds credentials for the Bluesky AT Protocol API.
type BlueskyConfig struct {
	Handle   string // e.g. "baskins.bsky.social"
	Password string // app password
	PDS      string // PDS host, defaults to https://bsky.social
}

// BlueskyClient posts to Bluesky via the AT Protocol.
type BlueskyClient struct {
	config     BlueskyConfig
	httpClient *http.Client

	// session state
	did        string
	accessJwt  string
	refreshJwt string
}

// NewBlueskyClient creates a client with the given config.
func NewBlueskyClient(config BlueskyConfig) *BlueskyClient {
	if config.PDS == "" {
		config.PDS = defaultPDS
	}
	return &BlueskyClient{
		config:     config,
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// Authenticate creates a session with the PDS.
func (c *BlueskyClient) Authenticate(ctx context.Context) error {
	body := map[string]string{
		"identifier": c.config.Handle,
		"password":   c.config.Password,
	}

	var resp struct {
		DID        string `json:"did"`
		AccessJwt  string `json:"accessJwt"`
		RefreshJwt string `json:"refreshJwt"`
	}

	if err := c.xrpc(ctx, "com.atproto.server.createSession", body, &resp); err != nil {
		return fmt.Errorf("create session: %w", err)
	}

	c.did = resp.DID
	c.accessJwt = resp.AccessJwt
	c.refreshJwt = resp.RefreshJwt
	return nil
}

// Post creates a post on Bluesky. Returns the AT URI and CID.
func (c *BlueskyClient) Post(ctx context.Context, text string) (uri string, cid string, err error) {
	record := map[string]any{
		"$type":     "app.bsky.feed.post",
		"text":      text,
		"createdAt": time.Now().UTC().Format(time.RFC3339Nano),
		"langs":     []string{"en"},
	}

	return c.createRecord(ctx, record)
}

// PostWithLink creates a post with a clickable link appended to the text.
func (c *BlueskyClient) PostWithLink(ctx context.Context, text, linkURL, linkText string) (uri string, cid string, err error) {
	if linkText == "" {
		linkText = linkURL
	}

	fullText := text + "\n\n" + linkText
	byteStart := len([]byte(text + "\n\n"))
	byteEnd := byteStart + len([]byte(linkText))

	record := map[string]any{
		"$type":     "app.bsky.feed.post",
		"text":      fullText,
		"createdAt": time.Now().UTC().Format(time.RFC3339Nano),
		"langs":     []string{"en"},
		"facets": []map[string]any{
			{
				"index": map[string]int{
					"byteStart": byteStart,
					"byteEnd":   byteEnd,
				},
				"features": []map[string]any{
					{
						"$type": "app.bsky.richtext.facet#link",
						"uri":   linkURL,
					},
				},
			},
		},
	}

	return c.createRecord(ctx, record)
}

// PostWithImage creates a post with an attached image.
func (c *BlueskyClient) PostWithImage(ctx context.Context, text, imagePath, altText string) (uri string, cid string, err error) {
	blob, err := c.uploadImage(ctx, imagePath)
	if err != nil {
		return "", "", fmt.Errorf("upload image: %w", err)
	}

	record := map[string]any{
		"$type":     "app.bsky.feed.post",
		"text":      text,
		"createdAt": time.Now().UTC().Format(time.RFC3339Nano),
		"langs":     []string{"en"},
		"embed": map[string]any{
			"$type": "app.bsky.embed.images",
			"images": []map[string]any{
				{
					"alt":   altText,
					"image": blob,
				},
			},
		},
	}

	return c.createRecord(ctx, record)
}

func (c *BlueskyClient) createRecord(ctx context.Context, record map[string]any) (string, string, error) {
	body := map[string]any{
		"repo":       c.did,
		"collection": "app.bsky.feed.post",
		"record":     record,
	}

	var resp struct {
		URI string `json:"uri"`
		CID string `json:"cid"`
	}

	if err := c.xrpcAuth(ctx, "com.atproto.repo.createRecord", body, &resp); err != nil {
		return "", "", fmt.Errorf("create record: %w", err)
	}

	return resp.URI, resp.CID, nil
}

func (c *BlueskyClient) uploadImage(ctx context.Context, path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	mimeType := "image/jpeg"
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".png"):
		mimeType = "image/png"
	case strings.HasSuffix(lower, ".webp"):
		mimeType = "image/webp"
	case strings.HasSuffix(lower, ".gif"):
		mimeType = "image/gif"
	}

	url := c.config.PDS + "/xrpc/com.atproto.repo.uploadBlob"
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", mimeType)
	req.Header.Set("Authorization", "Bearer "+c.accessJwt)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("uploadBlob %d: %s", resp.StatusCode, body)
	}

	var result struct {
		Blob map[string]any `json:"blob"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("decode uploadBlob response: %w", err)
	}

	return result.Blob, nil
}

// xrpc makes an unauthenticated XRPC call.
func (c *BlueskyClient) xrpc(ctx context.Context, method string, body any, result any) error {
	return c.doXRPC(ctx, method, body, result, "")
}

// xrpcAuth makes an authenticated XRPC call.
func (c *BlueskyClient) xrpcAuth(ctx context.Context, method string, body any, result any) error {
	return c.doXRPC(ctx, method, body, result, c.accessJwt)
}

func (c *BlueskyClient) doXRPC(ctx context.Context, method string, body any, result any, token string) error {
	data, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	url := c.config.PDS + "/xrpc/" + method
	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("xrpc %s %d: %s", method, resp.StatusCode, respBody)
	}

	if result != nil {
		if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
			return fmt.Errorf("decode %s response: %w", method, err)
		}
	}

	return nil
}

// BlueskyPostURL converts an AT URI to a web URL.
// at://did:plc:abc/app.bsky.feed.post/xyz → https://bsky.app/profile/handle/post/xyz
func BlueskyPostURL(handle, atURI string) string {
	// at://did:plc:abc/app.bsky.feed.post/rkey
	parts := strings.Split(atURI, "/")
	if len(parts) < 5 {
		return atURI
	}
	rkey := parts[len(parts)-1]
	return fmt.Sprintf("https://bsky.app/profile/%s/post/%s", handle, rkey)
}
