package synd

import (
	"encoding/json"
	"testing"
	"time"
)

func TestPostCreatedRoundTrip(t *testing.T) {
	now := time.Date(2026, 3, 13, 12, 0, 0, 0, time.UTC)
	orig := PostCreated{
		ID:        "post-001",
		Kind:      Short,
		Body:      "thinking about federation",
		Tags:      []string{"fediverse", "syndication"},
		CreatedAt: now,
	}

	data := MarshalData(orig)

	var decoded PostCreated
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != orig.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, orig.ID)
	}
	if decoded.Kind != orig.Kind {
		t.Errorf("Kind = %q, want %q", decoded.Kind, orig.Kind)
	}
	if decoded.Body != orig.Body {
		t.Errorf("Body = %q, want %q", decoded.Body, orig.Body)
	}
	if len(decoded.Tags) != 2 || decoded.Tags[0] != "fediverse" {
		t.Errorf("Tags = %v, want [fediverse syndication]", decoded.Tags)
	}
	if !decoded.CreatedAt.Equal(orig.CreatedAt) {
		t.Errorf("CreatedAt = %v, want %v", decoded.CreatedAt, orig.CreatedAt)
	}
}

func TestPostPublishedRoundTrip(t *testing.T) {
	now := time.Date(2026, 3, 13, 12, 5, 0, 0, time.UTC)
	orig := PostPublished{
		ID:          "post-001",
		URL:         "https://generativeplane.com/posts/post-001",
		PublishedAt: now,
	}

	data := MarshalData(orig)

	var decoded PostPublished
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != orig.ID {
		t.Errorf("ID = %q, want %q", decoded.ID, orig.ID)
	}
	if decoded.URL != orig.URL {
		t.Errorf("URL = %q, want %q", decoded.URL, orig.URL)
	}
	if !decoded.PublishedAt.Equal(orig.PublishedAt) {
		t.Errorf("PublishedAt = %v, want %v", decoded.PublishedAt, orig.PublishedAt)
	}
}

func TestPostSyndicatedRoundTrip(t *testing.T) {
	now := time.Date(2026, 3, 13, 12, 10, 0, 0, time.UTC)
	orig := PostSyndicated{
		PostID:    "post-001",
		Platform:  Bluesky,
		RemoteID:  "at://did:plc:abc123/app.bsky.feed.post/xyz",
		RemoteURL: "https://bsky.app/profile/ben.bsky.social/post/xyz",
		CreatedAt: now,
	}

	data := MarshalData(orig)

	var decoded PostSyndicated
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.PostID != orig.PostID {
		t.Errorf("PostID = %q, want %q", decoded.PostID, orig.PostID)
	}
	if decoded.Platform != orig.Platform {
		t.Errorf("Platform = %q, want %q", decoded.Platform, orig.Platform)
	}
	if decoded.RemoteID != orig.RemoteID {
		t.Errorf("RemoteID = %q, want %q", decoded.RemoteID, orig.RemoteID)
	}
	if decoded.RemoteURL != orig.RemoteURL {
		t.Errorf("RemoteURL = %q, want %q", decoded.RemoteURL, orig.RemoteURL)
	}
}

func TestPostEngagementUpdatedRoundTrip(t *testing.T) {
	now := time.Date(2026, 3, 13, 14, 0, 0, 0, time.UTC)
	orig := PostEngagementUpdated{
		PostID:    "post-001",
		Platform:  Mastodon,
		Likes:     42,
		Reposts:   7,
		Replies:   3,
		Views:     0,
		FetchedAt: now,
	}

	data := MarshalData(orig)

	var decoded PostEngagementUpdated
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Likes != 42 {
		t.Errorf("Likes = %d, want 42", decoded.Likes)
	}
	if decoded.Reposts != 7 {
		t.Errorf("Reposts = %d, want 7", decoded.Reposts)
	}
	if decoded.Replies != 3 {
		t.Errorf("Replies = %d, want 3", decoded.Replies)
	}
	if decoded.Platform != Mastodon {
		t.Errorf("Platform = %q, want %q", decoded.Platform, Mastodon)
	}
}

func TestLongFormPostCreated(t *testing.T) {
	orig := PostCreated{
		ID:        "post-002",
		Kind:      Long,
		Title:     "On Federation",
		Abstract:  "Why owning your content matters more than ever.",
		Body:      "# On Federation\n\nLong form markdown content...",
		Tags:      []string{"federation"},
		CreatedAt: time.Now().UTC(),
	}

	data := MarshalData(orig)

	var decoded PostCreated
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Kind != Long {
		t.Errorf("Kind = %q, want %q", decoded.Kind, Long)
	}
	if decoded.Title != orig.Title {
		t.Errorf("Title = %q, want %q", decoded.Title, orig.Title)
	}
	if decoded.Abstract != orig.Abstract {
		t.Errorf("Abstract = %q, want %q", decoded.Abstract, orig.Abstract)
	}
}

func TestImagePostCreated(t *testing.T) {
	orig := PostCreated{
		ID:        "post-003",
		Kind:      Image,
		Body:      "studio session",
		ImagePath: "/path/to/photo.png",
		CreatedAt: time.Now().UTC(),
	}

	data := MarshalData(orig)

	var decoded PostCreated
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.Kind != Image {
		t.Errorf("Kind = %q, want %q", decoded.Kind, Image)
	}
	if decoded.ImagePath != orig.ImagePath {
		t.Errorf("ImagePath = %q, want %q", decoded.ImagePath, orig.ImagePath)
	}
}

func TestImportedPostCreated(t *testing.T) {
	orig := PostCreated{
		ID:           "imported-001",
		Kind:         Short,
		Body:         "an old tweet",
		ImportedFrom: "twitter",
		CreatedAt:    time.Date(2020, 6, 15, 9, 0, 0, 0, time.UTC),
	}

	data := MarshalData(orig)

	var decoded PostCreated
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ImportedFrom != "twitter" {
		t.Errorf("ImportedFrom = %q, want %q", decoded.ImportedFrom, "twitter")
	}
}
