package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

// authToken returns the CLI session token, auto-provisioning one if needed.
func authToken() (string, error) {
	if tok := os.Getenv("SYND_AUTH_TOKEN"); tok != "" {
		return tok, nil
	}

	tokenPath := tokenFilePath()
	if data, err := os.ReadFile(tokenPath); err == nil {
		return string(data), nil
	}

	// Auto-provision a service-user token
	authURL := os.Getenv("SYND_AUTH_URL")
	if authURL == "" {
		authURL = "https://auth.studio.internal"
	}

	tok, err := provisionServiceUser(authURL)
	if err != nil {
		return "", fmt.Errorf("auto-provision auth token: %w", err)
	}

	// Save for subsequent runs
	if err := os.MkdirAll(filepath.Dir(tokenPath), 0o700); err != nil {
		return tok, nil // have token, just can't persist
	}
	os.WriteFile(tokenPath, []byte(tok), 0o600)

	return tok, nil
}

func tokenFilePath() string {
	if dir := os.Getenv("XDG_CONFIG_HOME"); dir != "" {
		return filepath.Join(dir, "synd", "token")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "synd", "token")
}

func provisionServiceUser(authURL string) (string, error) {
	body, _ := json.Marshal(map[string]string{
		"username":     "synd-cli",
		"display_name": "Synd CLI",
	})

	resp, err := http.Post(authURL+"/internal/service-user", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("POST %s/internal/service-user: %w", authURL, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", fmt.Errorf("service-user provision returned %d", resp.StatusCode)
	}

	var result struct {
		SessionToken string `json:"session_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode service-user response: %w", err)
	}
	if result.SessionToken == "" {
		return "", fmt.Errorf("empty session_token in response")
	}

	return result.SessionToken, nil
}

// authedRequest creates an HTTP request with the session cookie attached.
func authedRequest(method, url string, body *bytes.Reader) (*http.Request, error) {
	tok, err := authToken()
	if err != nil {
		return nil, err
	}

	var req *http.Request
	if body != nil {
		req, err = http.NewRequest(method, url, body)
	} else {
		req, err = http.NewRequest(method, url, nil)
	}
	if err != nil {
		return nil, err
	}

	req.AddCookie(&http.Cookie{Name: "session", Value: tok})
	return req, nil
}
