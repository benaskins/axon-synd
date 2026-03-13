// Package synd provides a personal syndication engine: publish posts to a
// canonical static site and syndicate copies to social platforms.
package synd

import (
	"encoding/json"
	"time"
)

// PostKind distinguishes the shape of a post.
type PostKind string

const (
	Short PostKind = "short"
	Long  PostKind = "long"
	Image PostKind = "image"
)

// Post is the canonical representation of a published piece of content.
type Post struct {
	ID        string   `json:"id"`
	Kind      PostKind `json:"kind"`
	Title     string   `json:"title,omitempty"`
	Abstract  string   `json:"abstract,omitempty"`
	Body      string   `json:"body"`
	ImagePath string   `json:"image_path,omitempty"`
	Tags      []string `json:"tags,omitempty"`

	// ImportedFrom records the source platform for archived posts.
	// Empty for posts authored locally.
	ImportedFrom string `json:"imported_from,omitempty"`

	CreatedAt   time.Time `json:"created_at"`
	PublishedAt time.Time `json:"published_at,omitempty"`
}

// SyndicationRecord tracks a post's presence on an external platform.
type SyndicationRecord struct {
	PostID     string    `json:"post_id"`
	Platform   string    `json:"platform"`
	RemoteID   string    `json:"remote_id"`
	RemoteURL  string    `json:"remote_url,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

// Engagement holds metrics for a single post on a single platform.
type Engagement struct {
	PostID    string    `json:"post_id"`
	Platform  string    `json:"platform"`
	Likes     int       `json:"likes"`
	Reposts   int       `json:"reposts"`
	Replies   int       `json:"replies"`
	Views     int       `json:"views,omitempty"`
	FetchedAt time.Time `json:"fetched_at"`
}

// Platform identifies a syndication target.
type Platform string

const (
	Bluesky  Platform = "bluesky"
	Mastodon Platform = "mastodon"
	Threads  Platform = "threads"
)

// Event types for post lifecycle.
const (
	EventPostCreated          = "post.created"
	EventPostPublished        = "post.published"
	EventPostSyndicated       = "post.syndicated"
	EventPostEngagementUpdate = "post.engagement_updated"
)

// PostCreated is emitted when a new post is authored.
type PostCreated struct {
	ID           string   `json:"id"`
	Kind         PostKind `json:"kind"`
	Title        string   `json:"title,omitempty"`
	Abstract     string   `json:"abstract,omitempty"`
	Body         string   `json:"body"`
	ImagePath    string   `json:"image_path,omitempty"`
	Tags         []string `json:"tags,omitempty"`
	ImportedFrom string   `json:"imported_from,omitempty"`
	CreatedAt    time.Time `json:"created_at"`
}

// PostPublished is emitted when the static site is rebuilt and pushed.
type PostPublished struct {
	ID          string    `json:"id"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"published_at"`
}

// PostSyndicated is emitted when a copy is sent to an external platform.
type PostSyndicated struct {
	PostID    string   `json:"post_id"`
	Platform  Platform `json:"platform"`
	RemoteID  string   `json:"remote_id"`
	RemoteURL string   `json:"remote_url,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// PostEngagementUpdated is emitted when metrics are polled from a platform.
type PostEngagementUpdated struct {
	PostID    string   `json:"post_id"`
	Platform  Platform `json:"platform"`
	Likes     int      `json:"likes"`
	Reposts   int      `json:"reposts"`
	Replies   int      `json:"replies"`
	Views     int      `json:"views,omitempty"`
	FetchedAt time.Time `json:"fetched_at"`
}

// MarshalData serialises an event payload to JSON.
func MarshalData(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}
