package main

import (
	"fmt"
	"net/url"
	"os"

	gate "github.com/benaskins/axon-gate"
	synd "github.com/benaskins/axon-synd"
)

// signalClientFromEnv returns a SignalClient if env vars are set.
// SYND_SIGNAL_URL: Signal REST API (e.g. http://localhost:8093)
// SYND_SIGNAL_RECIPIENT: phone number to notify (e.g. +61400000000)
func signalClientFromEnv() (*gate.SignalClient, bool) {
	apiURL := os.Getenv("SYND_SIGNAL_URL")
	recipient := os.Getenv("SYND_SIGNAL_RECIPIENT")
	if apiURL == "" || recipient == "" {
		return nil, false
	}
	return gate.NewSignalClient(apiURL, recipient), true
}

// sendDraftNotification sends a Signal message about a new draft post.
func sendDraftNotification(signal *gate.SignalClient, post *synd.Post, reviewBaseURL string) error {
	body := post.Body
	if post.Kind == synd.Long && post.Title != "" {
		body = post.Title
	}
	if len(body) > 80 {
		body = body[:77] + "..."
	}

	reviewURL := fmt.Sprintf("%s/drafts/%s?token=%s", reviewBaseURL, post.ID, url.QueryEscape(post.ApprovalToken))

	message := fmt.Sprintf("New draft for review\n\nKind: %s\n%s\n\n%s",
		post.Kind, body, reviewURL)

	return signal.Send(message)
}
