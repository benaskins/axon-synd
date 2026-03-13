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

// PostStatus tracks where a post is in its lifecycle.
type PostStatus string

const (
	StatusDraft     PostStatus = "draft"
	StatusApproved  PostStatus = "approved"
	StatusPublished PostStatus = "published"
	StatusDeleted   PostStatus = "deleted"
)

// Post is the canonical representation of a piece of content.
type Post struct {
	ID        string     `json:"id"`
	Kind      PostKind   `json:"kind"`
	Status    PostStatus `json:"status"`
	Title     string     `json:"title,omitempty"`
	Abstract  string     `json:"abstract,omitempty"`
	Body      string     `json:"body"`
	ImagePath string     `json:"image_path,omitempty"`
	Tags      []string   `json:"tags,omitempty"`

	// ImportedFrom records the source platform for archived posts.
	// Empty for posts authored locally.
	ImportedFrom string `json:"imported_from,omitempty"`

	// ApprovalToken is a one-time token for the approval gate URL.
	ApprovalToken string `json:"approval_token,omitempty"`

	CreatedAt   time.Time `json:"created_at"`
	ApprovedAt  time.Time `json:"approved_at,omitempty"`
	ApprovedBy  string    `json:"approved_by,omitempty"`
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
	EventPostRevised          = "post.revised"
	EventPostApproved         = "post.approved"
	EventPostPublished        = "post.published"
	EventPostSyndicated       = "post.syndicated"
	EventPostDeleted          = "post.deleted"
	EventPostEngagementUpdate = "post.engagement_updated"
)

// PostCreated is emitted when a new post is authored.
type PostCreated struct {
	ID            string   `json:"id"`
	Kind          PostKind `json:"kind"`
	Title         string   `json:"title,omitempty"`
	Abstract      string   `json:"abstract,omitempty"`
	Body          string   `json:"body"`
	ImagePath     string   `json:"image_path,omitempty"`
	Tags          []string `json:"tags,omitempty"`
	ImportedFrom  string   `json:"imported_from,omitempty"`
	ApprovalToken string   `json:"approval_token,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
}

// PostRevised is emitted when a draft post is edited.
type PostRevised struct {
	PostID    string    `json:"post_id"`
	Title     string    `json:"title,omitempty"`
	Abstract  string    `json:"abstract,omitempty"`
	Body      string    `json:"body"`
	Tags      []string  `json:"tags,omitempty"`
	RevisedAt time.Time `json:"revised_at"`
	RevisedBy string    `json:"revised_by,omitempty"`
}

// PostApproved is emitted when a human approves a draft for publishing.
type PostApproved struct {
	PostID     string    `json:"post_id"`
	ApprovedAt time.Time `json:"approved_at"`
	ApprovedBy string    `json:"approved_by"`
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

// PostDeleted is emitted when a post is removed from the site.
type PostDeleted struct {
	PostID    string    `json:"post_id"`
	DeletedAt time.Time `json:"deleted_at"`
	DeletedBy string    `json:"deleted_by"`
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
