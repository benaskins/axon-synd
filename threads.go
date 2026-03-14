package synd

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const threadsAPI = "https://graph.threads.net/v1.0"

// ThreadsConfig holds credentials for the Threads API.
type ThreadsConfig struct {
	AccessToken string // long-lived OAuth token
}

// ThreadsClient posts to Threads via Meta's Graph API.
type ThreadsClient struct {
	config     ThreadsConfig
	httpClient *http.Client

	// resolved on first call
	userID string
}

// NewThreadsClient creates a client with the given config.
func NewThreadsClient(config ThreadsConfig) *ThreadsClient {
	return &ThreadsClient{
		config:     config,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// VerifyCredentials fetches the authenticated user's ID.
func (c *ThreadsClient) VerifyCredentials(ctx context.Context) error {
	var me struct {
		ID string `json:"id"`
	}

	if err := c.get(ctx, "/me", url.Values{"fields": {"id"}}, &me); err != nil {
		return fmt.Errorf("verify credentials: %w", err)
	}

	c.userID = me.ID
	return nil
}

// Post creates a text post on Threads. Returns the published media ID.
func (c *ThreadsClient) Post(ctx context.Context, text string) (string, error) {
	containerID, err := c.createContainer(ctx, url.Values{
		"media_type": {"TEXT"},
		"text":       {text},
	})
	if err != nil {
		return "", err
	}

	return c.publish(ctx, containerID)
}

// PostWithLink creates a text post with a link appended.
func (c *ThreadsClient) PostWithLink(ctx context.Context, text, linkURL string) (string, error) {
	fullText := text + "\n\n" + linkURL
	return c.Post(ctx, fullText)
}

// PostWithImage creates a post with an attached image.
func (c *ThreadsClient) PostWithImage(ctx context.Context, text, imageURL string) (string, error) {
	containerID, err := c.createContainer(ctx, url.Values{
		"media_type": {"IMAGE"},
		"image_url":  {imageURL},
		"text":       {text},
	})
	if err != nil {
		return "", err
	}

	return c.publish(ctx, containerID)
}

func (c *ThreadsClient) createContainer(ctx context.Context, params url.Values) (string, error) {
	params.Set("access_token", c.config.AccessToken)

	var resp struct {
		ID string `json:"id"`
	}

	endpoint := fmt.Sprintf("/%s/threads", c.userID)
	if err := c.post(ctx, endpoint, params, &resp); err != nil {
		return "", fmt.Errorf("create container: %w", err)
	}

	return resp.ID, nil
}

func (c *ThreadsClient) publish(ctx context.Context, containerID string) (string, error) {
	// Threads recommends waiting for the container to be processed.
	time.Sleep(5 * time.Second)

	params := url.Values{
		"creation_id":  {containerID},
		"access_token": {c.config.AccessToken},
	}

	var resp struct {
		ID string `json:"id"`
	}

	endpoint := fmt.Sprintf("/%s/threads_publish", c.userID)
	if err := c.post(ctx, endpoint, params, &resp); err != nil {
		return "", fmt.Errorf("publish: %w", err)
	}

	return resp.ID, nil
}

func (c *ThreadsClient) get(ctx context.Context, path string, params url.Values, result any) error {
	params.Set("access_token", c.config.AccessToken)

	reqURL := threadsAPI + path + "?" + params.Encode()
	req, err := http.NewRequestWithContext(ctx, "GET", reqURL, nil)
	if err != nil {
		return err
	}

	return c.doRequest(req, result)
}

func (c *ThreadsClient) post(ctx context.Context, path string, params url.Values, result any) error {
	req, err := http.NewRequestWithContext(ctx, "POST", threadsAPI+path, strings.NewReader(params.Encode()))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return c.doRequest(req, result)
}

func (c *ThreadsClient) doRequest(req *http.Request, result any) error {
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

// ThreadsPostURL constructs the web URL for a Threads post.
func ThreadsPostURL(username, mediaID string) string {
	return fmt.Sprintf("https://www.threads.net/@%s/post/%s", username, mediaID)
}
