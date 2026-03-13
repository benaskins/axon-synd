package synd

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	fact "github.com/benaskins/axon-fact"
	"github.com/google/uuid"
)

// PostStore manages posts through an event-sourced append-only log.
type PostStore struct {
	events     fact.EventStore
	projection *PostProjection
}

// NewPostStore creates a post store backed by the given event store.
// The projection is registered as a projector so it stays in sync.
func NewPostStore(events fact.EventStore) *PostStore {
	return &PostStore{
		events:     events,
		projection: &PostProjection{},
	}
}

// Projection returns the read model for direct query access.
func (s *PostStore) Projection() *PostProjection {
	return s.projection
}

// Projector returns the projector for registration with the event store.
func (s *PostStore) Projector() fact.Projector {
	return s.projection
}

// SetEventStore replaces the backing event store. Used when the store
// is constructed before the event store is available.
func (s *PostStore) SetEventStore(es fact.EventStore) {
	s.events = es
}

// Create persists a new post as a PostCreated event.
func (s *PostStore) Create(ctx context.Context, kind PostKind, body string, opts ...PostOption) (*Post, error) {
	cfg := postConfig{}
	for _, o := range opts {
		o(&cfg)
	}

	id := uuid.New().String()
	now := time.Now().UTC()

	created := PostCreated{
		ID:            id,
		Kind:          kind,
		Title:         cfg.title,
		Abstract:      cfg.abstract,
		Body:          body,
		ImagePath:     cfg.imagePath,
		Tags:          cfg.tags,
		ImportedFrom:  cfg.importedFrom,
		ApprovalToken: cfg.approvalToken,
		CreatedAt:     now,
	}

	event := fact.Event{
		ID:   uuid.New().String(),
		Type: EventPostCreated,
		Data: MarshalData(created),
	}

	if err := s.events.Append(ctx, streamKey(id), []fact.Event{event}); err != nil {
		return nil, fmt.Errorf("append post.created: %w", err)
	}

	post := s.projection.Get(id)
	if post == nil {
		return nil, fmt.Errorf("post %s not found after create", id)
	}
	return post, nil
}

// Revise updates a draft post's content.
func (s *PostStore) Revise(ctx context.Context, postID, body, title, abstract string, tags []string, revisedBy string) error {
	now := time.Now().UTC()

	revised := PostRevised{
		PostID:    postID,
		Title:     title,
		Abstract:  abstract,
		Body:      body,
		Tags:      tags,
		RevisedAt: now,
		RevisedBy: revisedBy,
	}

	event := fact.Event{
		ID:   uuid.New().String(),
		Type: EventPostRevised,
		Data: MarshalData(revised),
	}

	return s.events.Append(ctx, streamKey(postID), []fact.Event{event})
}

// Approve marks a draft post as approved for publishing.
func (s *PostStore) Approve(ctx context.Context, postID, approvedBy string) error {
	now := time.Now().UTC()

	approved := PostApproved{
		PostID:     postID,
		ApprovedAt: now,
		ApprovedBy: approvedBy,
	}

	event := fact.Event{
		ID:   uuid.New().String(),
		Type: EventPostApproved,
		Data: MarshalData(approved),
	}

	return s.events.Append(ctx, streamKey(postID), []fact.Event{event})
}

// Publish marks a post as published at the given URL.
func (s *PostStore) Publish(ctx context.Context, postID, url string) error {
	now := time.Now().UTC()

	published := PostPublished{
		ID:          postID,
		URL:         url,
		PublishedAt: now,
	}

	event := fact.Event{
		ID:   uuid.New().String(),
		Type: EventPostPublished,
		Data: MarshalData(published),
	}

	return s.events.Append(ctx, streamKey(postID), []fact.Event{event})
}

// Syndicate records that a post was sent to a platform.
func (s *PostStore) Syndicate(ctx context.Context, postID string, platform Platform, remoteID, remoteURL string) error {
	now := time.Now().UTC()

	syndicated := PostSyndicated{
		PostID:    postID,
		Platform:  platform,
		RemoteID:  remoteID,
		RemoteURL: remoteURL,
		CreatedAt: now,
	}

	event := fact.Event{
		ID:   uuid.New().String(),
		Type: EventPostSyndicated,
		Data: MarshalData(syndicated),
	}

	return s.events.Append(ctx, streamKey(postID), []fact.Event{event})
}

// Delete marks a post as deleted so it is excluded from listings and site builds.
func (s *PostStore) Delete(ctx context.Context, postID, deletedBy string) error {
	now := time.Now().UTC()

	deleted := PostDeleted{
		PostID:    postID,
		DeletedAt: now,
		DeletedBy: deletedBy,
	}

	event := fact.Event{
		ID:   uuid.New().String(),
		Type: EventPostDeleted,
		Data: MarshalData(deleted),
	}

	return s.events.Append(ctx, streamKey(postID), []fact.Event{event})
}

// UpdateEngagement records polled metrics for a post on a platform.
func (s *PostStore) UpdateEngagement(ctx context.Context, postID string, platform Platform, likes, reposts, replies, views int) error {
	now := time.Now().UTC()

	updated := PostEngagementUpdated{
		PostID:    postID,
		Platform:  platform,
		Likes:     likes,
		Reposts:   reposts,
		Replies:   replies,
		Views:     views,
		FetchedAt: now,
	}

	event := fact.Event{
		ID:   uuid.New().String(),
		Type: EventPostEngagementUpdate,
		Data: MarshalData(updated),
	}

	return s.events.Append(ctx, streamKey(postID), []fact.Event{event})
}

// Get returns a post by ID.
func (s *PostStore) Get(id string) *Post {
	return s.projection.Get(id)
}

// List returns all posts in reverse chronological order.
func (s *PostStore) List() []Post {
	return s.projection.List()
}

func streamKey(postID string) string {
	return "post-" + postID
}

// PostOption configures optional fields when creating a post.
type PostOption func(*postConfig)

type postConfig struct {
	title         string
	abstract      string
	imagePath     string
	tags          []string
	importedFrom  string
	approvalToken string
}

func WithTitle(t string) PostOption        { return func(c *postConfig) { c.title = t } }
func WithAbstract(a string) PostOption     { return func(c *postConfig) { c.abstract = a } }
func WithImagePath(p string) PostOption    { return func(c *postConfig) { c.imagePath = p } }
func WithTags(t ...string) PostOption      { return func(c *postConfig) { c.tags = t } }
func WithImportedFrom(p string) PostOption  { return func(c *postConfig) { c.importedFrom = p } }
func WithApprovalToken(t string) PostOption { return func(c *postConfig) { c.approvalToken = t } }

// PostProjection is a read model built from post events.
type PostProjection struct {
	mu    sync.RWMutex
	posts map[string]*Post
	synd  map[string][]SyndicationRecord
	eng   map[string]map[Platform]*Engagement
}

// Handle processes a single event to update the projection.
func (p *PostProjection) Handle(_ context.Context, event fact.Event) error {
	p.init()

	switch event.Type {
	case EventPostCreated:
		var data PostCreated
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return fmt.Errorf("unmarshal post.created: %w", err)
		}
		p.mu.Lock()
		p.posts[data.ID] = &Post{
			ID:            data.ID,
			Kind:          data.Kind,
			Status:        StatusDraft,
			Title:         data.Title,
			Abstract:      data.Abstract,
			Body:          data.Body,
			ImagePath:     data.ImagePath,
			Tags:          data.Tags,
			ImportedFrom:  data.ImportedFrom,
			ApprovalToken: data.ApprovalToken,
			CreatedAt:     data.CreatedAt,
		}
		p.mu.Unlock()

	case EventPostRevised:
		var data PostRevised
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return fmt.Errorf("unmarshal post.revised: %w", err)
		}
		p.mu.Lock()
		if post, ok := p.posts[data.PostID]; ok {
			post.Body = data.Body
			if data.Title != "" {
				post.Title = data.Title
			}
			if data.Abstract != "" {
				post.Abstract = data.Abstract
			}
			if data.Tags != nil {
				post.Tags = data.Tags
			}
		}
		p.mu.Unlock()

	case EventPostApproved:
		var data PostApproved
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return fmt.Errorf("unmarshal post.approved: %w", err)
		}
		p.mu.Lock()
		if post, ok := p.posts[data.PostID]; ok {
			post.Status = StatusApproved
			post.ApprovedAt = data.ApprovedAt
			post.ApprovedBy = data.ApprovedBy
		}
		p.mu.Unlock()

	case EventPostPublished:
		var data PostPublished
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return fmt.Errorf("unmarshal post.published: %w", err)
		}
		p.mu.Lock()
		if post, ok := p.posts[data.ID]; ok {
			post.Status = StatusPublished
			post.PublishedAt = data.PublishedAt
		}
		p.mu.Unlock()

	case EventPostDeleted:
		var data PostDeleted
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return fmt.Errorf("unmarshal post.deleted: %w", err)
		}
		p.mu.Lock()
		if post, ok := p.posts[data.PostID]; ok {
			post.Status = StatusDeleted
		}
		p.mu.Unlock()

	case EventPostSyndicated:
		var data PostSyndicated
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return fmt.Errorf("unmarshal post.syndicated: %w", err)
		}
		p.mu.Lock()
		p.synd[data.PostID] = append(p.synd[data.PostID], SyndicationRecord{
			PostID:    data.PostID,
			Platform:  string(data.Platform),
			RemoteID:  data.RemoteID,
			RemoteURL: data.RemoteURL,
			CreatedAt: data.CreatedAt,
		})
		p.mu.Unlock()

	case EventPostEngagementUpdate:
		var data PostEngagementUpdated
		if err := json.Unmarshal(event.Data, &data); err != nil {
			return fmt.Errorf("unmarshal post.engagement_updated: %w", err)
		}
		p.mu.Lock()
		if _, ok := p.eng[data.PostID]; !ok {
			p.eng[data.PostID] = make(map[Platform]*Engagement)
		}
		p.eng[data.PostID][data.Platform] = &Engagement{
			PostID:    data.PostID,
			Platform:  string(data.Platform),
			Likes:     data.Likes,
			Reposts:   data.Reposts,
			Replies:   data.Replies,
			Views:     data.Views,
			FetchedAt: data.FetchedAt,
		}
		p.mu.Unlock()
	}

	return nil
}

// Get returns a post by ID, or nil if not found.
func (p *PostProjection) Get(id string) *Post {
	p.init()
	p.mu.RLock()
	defer p.mu.RUnlock()
	post, ok := p.posts[id]
	if !ok {
		return nil
	}
	cp := *post
	return &cp
}

// List returns all posts sorted by creation time, newest first.
func (p *PostProjection) List() []Post {
	p.init()
	p.mu.RLock()
	defer p.mu.RUnlock()

	posts := make([]Post, 0, len(p.posts))
	for _, post := range p.posts {
		if post.Status == StatusDeleted {
			continue
		}
		posts = append(posts, *post)
	}

	// Sort newest first
	for i := 0; i < len(posts); i++ {
		for j := i + 1; j < len(posts); j++ {
			if posts[j].CreatedAt.After(posts[i].CreatedAt) {
				posts[i], posts[j] = posts[j], posts[i]
			}
		}
	}

	return posts
}

// Syndications returns the syndication records for a post.
func (p *PostProjection) Syndications(postID string) []SyndicationRecord {
	p.init()
	p.mu.RLock()
	defer p.mu.RUnlock()
	records := p.synd[postID]
	out := make([]SyndicationRecord, len(records))
	copy(out, records)
	return out
}

// EngagementFor returns engagement metrics for a post across all platforms.
func (p *PostProjection) EngagementFor(postID string) []Engagement {
	p.init()
	p.mu.RLock()
	defer p.mu.RUnlock()
	platforms, ok := p.eng[postID]
	if !ok {
		return nil
	}
	out := make([]Engagement, 0, len(platforms))
	for _, e := range platforms {
		out = append(out, *e)
	}
	return out
}

// Drafts returns posts with status == draft.
func (p *PostProjection) Drafts() []Post {
	p.init()
	p.mu.RLock()
	defer p.mu.RUnlock()

	var out []Post
	for _, post := range p.posts {
		if post.Status == StatusDraft {
			out = append(out, *post)
		}
	}
	return out
}

// ApprovedPosts returns posts that are approved but not yet published.
func (p *PostProjection) ApprovedPosts() []Post {
	p.init()
	p.mu.RLock()
	defer p.mu.RUnlock()

	var out []Post
	for _, post := range p.posts {
		if post.Status == StatusApproved {
			out = append(out, *post)
		}
	}
	return out
}

// UnsyncedPosts returns posts that haven't been syndicated to the given platform.
func (p *PostProjection) UnsyncedPosts(platform Platform) []Post {
	p.init()
	p.mu.RLock()
	defer p.mu.RUnlock()

	var out []Post
	for id, post := range p.posts {
		if post.Status == StatusDeleted {
			continue
		}
		if post.ImportedFrom == string(platform) {
			continue
		}
		if post.PublishedAt.IsZero() {
			continue
		}
		synced := false
		for _, rec := range p.synd[id] {
			if rec.Platform == string(platform) {
				synced = true
				break
			}
		}
		if !synced {
			out = append(out, *post)
		}
	}
	return out
}

func (p *PostProjection) init() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.posts == nil {
		p.posts = make(map[string]*Post)
	}
	if p.synd == nil {
		p.synd = make(map[string][]SyndicationRecord)
	}
	if p.eng == nil {
		p.eng = make(map[string]map[Platform]*Engagement)
	}
}
