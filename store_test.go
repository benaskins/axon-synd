package synd

import (
	"context"
	"testing"

	fact "github.com/benaskins/axon-fact"
)

func newTestStore(t *testing.T) *PostStore {
	t.Helper()
	store := NewPostStore(nil)
	events := fact.NewMemoryStore(fact.WithProjector(store.Projector()))
	store.events = events
	return store
}

func TestPostStore_CreateShort(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	post, err := store.Create(ctx, Short, "hello world", WithTags("test"))
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if post.ID == "" {
		t.Error("expected non-empty ID")
	}
	if post.Kind != Short {
		t.Errorf("Kind = %q, want %q", post.Kind, Short)
	}
	if post.Body != "hello world" {
		t.Errorf("Body = %q, want %q", post.Body, "hello world")
	}
	if len(post.Tags) != 1 || post.Tags[0] != "test" {
		t.Errorf("Tags = %v, want [test]", post.Tags)
	}
	if post.CreatedAt.IsZero() {
		t.Error("expected non-zero CreatedAt")
	}
}

func TestPostStore_CreateLong(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	post, err := store.Create(ctx, Long, "# Article\n\nFull content here.",
		WithTitle("My Article"),
		WithAbstract("A short summary."),
	)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if post.Kind != Long {
		t.Errorf("Kind = %q, want %q", post.Kind, Long)
	}
	if post.Title != "My Article" {
		t.Errorf("Title = %q, want %q", post.Title, "My Article")
	}
	if post.Abstract != "A short summary." {
		t.Errorf("Abstract = %q, want %q", post.Abstract, "A short summary.")
	}
}

func TestPostStore_CreateImage(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	post, err := store.Create(ctx, Image, "studio session",
		WithImagePath("/photos/studio.png"),
	)
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if post.Kind != Image {
		t.Errorf("Kind = %q, want %q", post.Kind, Image)
	}
	if post.ImagePath != "/photos/studio.png" {
		t.Errorf("ImagePath = %q, want %q", post.ImagePath, "/photos/studio.png")
	}
}

func TestPostStore_Get(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	post, _ := store.Create(ctx, Short, "findable")
	got := store.Get(post.ID)
	if got == nil {
		t.Fatal("Get returned nil")
	}
	if got.Body != "findable" {
		t.Errorf("Body = %q, want %q", got.Body, "findable")
	}
}

func TestPostStore_GetNotFound(t *testing.T) {
	store := newTestStore(t)
	if store.Get("nonexistent") != nil {
		t.Error("expected nil for nonexistent post")
	}
}

func TestPostStore_List(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	store.Create(ctx, Short, "first")
	store.Create(ctx, Short, "second")
	store.Create(ctx, Short, "third")

	posts := store.List()
	if len(posts) != 3 {
		t.Fatalf("got %d posts, want 3", len(posts))
	}

	// Newest first
	if posts[0].Body != "third" {
		t.Errorf("first post = %q, want %q", posts[0].Body, "third")
	}
	if posts[2].Body != "first" {
		t.Errorf("last post = %q, want %q", posts[2].Body, "first")
	}
}

func TestPostStore_Publish(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	post, _ := store.Create(ctx, Short, "to be published")
	if err := store.Publish(ctx, post.ID, "https://generativeplane.com/posts/"+post.ID); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	got := store.Get(post.ID)
	if got.PublishedAt.IsZero() {
		t.Error("expected non-zero PublishedAt after publish")
	}
}

func TestPostStore_Syndicate(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	post, _ := store.Create(ctx, Short, "syndicate me")
	store.Publish(ctx, post.ID, "https://generativeplane.com/posts/"+post.ID)

	err := store.Syndicate(ctx, post.ID, Bluesky, "at://did:plc:abc/post/123", "https://bsky.app/post/123")
	if err != nil {
		t.Fatalf("Syndicate: %v", err)
	}

	records := store.Projection().Syndications(post.ID)
	if len(records) != 1 {
		t.Fatalf("got %d syndication records, want 1", len(records))
	}
	if records[0].Platform != string(Bluesky) {
		t.Errorf("Platform = %q, want %q", records[0].Platform, Bluesky)
	}
	if records[0].RemoteID != "at://did:plc:abc/post/123" {
		t.Errorf("RemoteID = %q", records[0].RemoteID)
	}
}

func TestPostStore_SyndicateMultiplePlatforms(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	post, _ := store.Create(ctx, Short, "everywhere")
	store.Publish(ctx, post.ID, "https://generativeplane.com/posts/"+post.ID)
	store.Syndicate(ctx, post.ID, Bluesky, "bsky-123", "")
	store.Syndicate(ctx, post.ID, Mastodon, "masto-456", "")

	records := store.Projection().Syndications(post.ID)
	if len(records) != 2 {
		t.Fatalf("got %d syndication records, want 2", len(records))
	}
}

func TestPostStore_UpdateEngagement(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	post, _ := store.Create(ctx, Short, "popular post")
	store.Syndicate(ctx, post.ID, Bluesky, "bsky-123", "")

	err := store.UpdateEngagement(ctx, post.ID, Bluesky, 42, 7, 3, 1000)
	if err != nil {
		t.Fatalf("UpdateEngagement: %v", err)
	}

	engagement := store.Projection().EngagementFor(post.ID)
	if len(engagement) != 1 {
		t.Fatalf("got %d engagement records, want 1", len(engagement))
	}
	if engagement[0].Likes != 42 {
		t.Errorf("Likes = %d, want 42", engagement[0].Likes)
	}
	if engagement[0].Views != 1000 {
		t.Errorf("Views = %d, want 1000", engagement[0].Views)
	}
}

func TestPostStore_EngagementUpdatesReplace(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	post, _ := store.Create(ctx, Short, "growing post")
	store.UpdateEngagement(ctx, post.ID, Bluesky, 10, 2, 1, 100)
	store.UpdateEngagement(ctx, post.ID, Bluesky, 42, 7, 3, 1000)

	engagement := store.Projection().EngagementFor(post.ID)
	if len(engagement) != 1 {
		t.Fatalf("got %d records, want 1 (latest replaces)", len(engagement))
	}
	if engagement[0].Likes != 42 {
		t.Errorf("Likes = %d, want 42 (latest)", engagement[0].Likes)
	}
}

func TestPostStore_UnsyncedPosts(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	p1, _ := store.Create(ctx, Short, "synced to bluesky")
	store.Publish(ctx, p1.ID, "https://generativeplane.com/posts/"+p1.ID)
	store.Syndicate(ctx, p1.ID, Bluesky, "bsky-1", "")

	p2, _ := store.Create(ctx, Short, "not synced")
	store.Publish(ctx, p2.ID, "https://generativeplane.com/posts/"+p2.ID)

	// Unpublished post should not appear
	store.Create(ctx, Short, "draft")

	unsynced := store.Projection().UnsyncedPosts(Bluesky)
	if len(unsynced) != 1 {
		t.Fatalf("got %d unsynced posts, want 1", len(unsynced))
	}
	if unsynced[0].ID != p2.ID {
		t.Errorf("unsynced post = %q, want %q", unsynced[0].ID, p2.ID)
	}
}

func TestPostStore_CreateIsDraft(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	post, _ := store.Create(ctx, Short, "a draft")
	if post.Status != StatusDraft {
		t.Errorf("Status = %q, want %q", post.Status, StatusDraft)
	}
}

func TestPostStore_Revise(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	post, _ := store.Create(ctx, Short, "original text")

	err := store.Revise(ctx, post.ID, "revised text", "", "", nil, "ben")
	if err != nil {
		t.Fatalf("Revise: %v", err)
	}

	got := store.Get(post.ID)
	if got.Body != "revised text" {
		t.Errorf("Body = %q, want %q", got.Body, "revised text")
	}
	if got.Status != StatusDraft {
		t.Errorf("Status = %q, want %q after revise", got.Status, StatusDraft)
	}
}

func TestPostStore_ReviseLong(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	post, _ := store.Create(ctx, Long, "# Original\n\nContent",
		WithTitle("Original"),
		WithAbstract("First abstract"),
	)

	err := store.Revise(ctx, post.ID, "# Revised\n\nNew content", "Revised", "Better abstract", []string{"go"}, "ben")
	if err != nil {
		t.Fatalf("Revise: %v", err)
	}

	got := store.Get(post.ID)
	if got.Title != "Revised" {
		t.Errorf("Title = %q", got.Title)
	}
	if got.Abstract != "Better abstract" {
		t.Errorf("Abstract = %q", got.Abstract)
	}
	if got.Body != "# Revised\n\nNew content" {
		t.Errorf("Body = %q", got.Body)
	}
	if len(got.Tags) != 1 || got.Tags[0] != "go" {
		t.Errorf("Tags = %v", got.Tags)
	}
}

func TestPostStore_Approve(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	post, _ := store.Create(ctx, Short, "approve me")

	err := store.Approve(ctx, post.ID, "ben")
	if err != nil {
		t.Fatalf("Approve: %v", err)
	}

	got := store.Get(post.ID)
	if got.Status != StatusApproved {
		t.Errorf("Status = %q, want %q", got.Status, StatusApproved)
	}
	if got.ApprovedAt.IsZero() {
		t.Error("expected non-zero ApprovedAt")
	}
	if got.ApprovedBy != "ben" {
		t.Errorf("ApprovedBy = %q", got.ApprovedBy)
	}
}

func TestPostStore_PublishSetsStatus(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	post, _ := store.Create(ctx, Short, "publish me")
	store.Approve(ctx, post.ID, "ben")
	store.Publish(ctx, post.ID, "https://example.com/posts/"+post.ID)

	got := store.Get(post.ID)
	if got.Status != StatusPublished {
		t.Errorf("Status = %q, want %q", got.Status, StatusPublished)
	}
}

func TestPostProjection_Drafts(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	store.Create(ctx, Short, "draft one")
	store.Create(ctx, Short, "draft two")
	p3, _ := store.Create(ctx, Short, "approved")
	store.Approve(ctx, p3.ID, "ben")

	drafts := store.Projection().Drafts()
	if len(drafts) != 2 {
		t.Fatalf("got %d drafts, want 2", len(drafts))
	}
}

func TestPostProjection_ApprovedPosts(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	store.Create(ctx, Short, "still draft")
	p2, _ := store.Create(ctx, Short, "approved not published")
	store.Approve(ctx, p2.ID, "ben")
	p3, _ := store.Create(ctx, Short, "published")
	store.Approve(ctx, p3.ID, "ben")
	store.Publish(ctx, p3.ID, "https://example.com/posts/"+p3.ID)

	approved := store.Projection().ApprovedPosts()
	if len(approved) != 1 {
		t.Fatalf("got %d approved posts, want 1", len(approved))
	}
	if approved[0].ID != p2.ID {
		t.Errorf("approved post = %q, want %q", approved[0].ID, p2.ID)
	}
}

func TestPostStore_Delete(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	post, _ := store.Create(ctx, Short, "delete me")
	if err := store.Delete(ctx, post.ID, "ben"); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got := store.Get(post.ID)
	if got.Status != StatusDeleted {
		t.Errorf("Status = %q, want %q", got.Status, StatusDeleted)
	}
}

func TestPostStore_DeletedPostExcludedFromList(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	p1, _ := store.Create(ctx, Short, "keep me")
	p2, _ := store.Create(ctx, Short, "delete me")
	store.Delete(ctx, p2.ID, "ben")

	posts := store.Projection().List()
	if len(posts) != 1 {
		t.Fatalf("got %d posts, want 1", len(posts))
	}
	if posts[0].ID != p1.ID {
		t.Errorf("remaining post = %q, want %q", posts[0].ID, p1.ID)
	}
}

func TestPostStore_DeletedPostExcludedFromDrafts(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	store.Create(ctx, Short, "keep me")
	p2, _ := store.Create(ctx, Short, "delete me")
	store.Delete(ctx, p2.ID, "ben")

	drafts := store.Projection().Drafts()
	if len(drafts) != 1 {
		t.Fatalf("got %d drafts, want 1", len(drafts))
	}
}

func TestPostStore_ImportedPostSkipsSourcPlatform(t *testing.T) {
	store := newTestStore(t)
	ctx := context.Background()

	p, _ := store.Create(ctx, Short, "old tweet", WithImportedFrom("twitter"))
	store.Publish(ctx, p.ID, "https://generativeplane.com/posts/"+p.ID)

	// Should appear for bluesky (different platform)
	unsynced := store.Projection().UnsyncedPosts(Bluesky)
	if len(unsynced) != 1 {
		t.Fatalf("got %d unsynced for bluesky, want 1", len(unsynced))
	}
}
