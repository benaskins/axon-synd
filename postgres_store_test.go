package synd

import (
	"context"
	"os"
	"testing"

	"time"

	"github.com/benaskins/axon"
	fact "github.com/benaskins/axon-fact"
	"github.com/google/uuid"
)

func openTestEventStore(t *testing.T) *PostgresEventStore {
	t.Helper()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://localhost:5432/lamina?sslmode=disable"
	}

	db := axon.OpenTestDB(t, dsn, Migrations)

	projection := &PostProjection{}
	store := NewPostgresEventStore(db, WithPgProjector(projection))
	return store
}

func skipIfNoPostgres(t *testing.T) {
	t.Helper()
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://localhost:5432/lamina?sslmode=disable"
	}

	db, err := axon.OpenDB(dsn, "ping_test")
	if err != nil {
		t.Skipf("skipping postgres test: %v", err)
	}
	db.Close()
}

func TestPostgresEventStore_Append(t *testing.T) {
	skipIfNoPostgres(t)
	store := openTestEventStore(t)
	ctx := context.Background()

	stream := "post-" + uuid.New().String()
	events := []fact.Event{
		{
			ID:   uuid.New().String(),
			Type: EventPostCreated,
			Data: MarshalData(PostCreated{
				ID:   "p1",
				Kind: Short,
				Body: "hello world",
			}),
		},
	}

	if err := store.Append(ctx, stream, events); err != nil {
		t.Fatalf("Append: %v", err)
	}

	loaded, err := store.Load(ctx, stream)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded) != 1 {
		t.Fatalf("got %d events, want 1", len(loaded))
	}
	if loaded[0].Stream != stream {
		t.Errorf("Stream = %q, want %q", loaded[0].Stream, stream)
	}
	if loaded[0].Type != EventPostCreated {
		t.Errorf("Type = %q, want %q", loaded[0].Type, EventPostCreated)
	}
	if loaded[0].Sequence != 1 {
		t.Errorf("Sequence = %d, want 1", loaded[0].Sequence)
	}
	if loaded[0].OccurredAt.IsZero() {
		t.Error("expected non-zero OccurredAt")
	}
}

func TestPostgresEventStore_AppendMultiple(t *testing.T) {
	skipIfNoPostgres(t)
	store := openTestEventStore(t)
	ctx := context.Background()

	stream := "post-" + uuid.New().String()

	// First append
	events1 := []fact.Event{
		{ID: uuid.New().String(), Type: EventPostCreated, Data: MarshalData(PostCreated{ID: "p1", Kind: Short, Body: "first"})},
	}
	if err := store.Append(ctx, stream, events1); err != nil {
		t.Fatalf("Append 1: %v", err)
	}

	// Second append — sequence should continue
	events2 := []fact.Event{
		{ID: uuid.New().String(), Type: EventPostPublished, Data: MarshalData(PostPublished{ID: "p1", URL: "https://example.com/p1"})},
	}
	if err := store.Append(ctx, stream, events2); err != nil {
		t.Fatalf("Append 2: %v", err)
	}

	loaded, err := store.Load(ctx, stream)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("got %d events, want 2", len(loaded))
	}
	if loaded[0].Sequence != 1 {
		t.Errorf("event[0].Sequence = %d, want 1", loaded[0].Sequence)
	}
	if loaded[1].Sequence != 2 {
		t.Errorf("event[1].Sequence = %d, want 2", loaded[1].Sequence)
	}
}

func TestPostgresEventStore_LoadFrom(t *testing.T) {
	skipIfNoPostgres(t)
	store := openTestEventStore(t)
	ctx := context.Background()

	stream := "post-" + uuid.New().String()

	events := []fact.Event{
		{ID: uuid.New().String(), Type: EventPostCreated, Data: MarshalData(PostCreated{ID: "p1", Kind: Short, Body: "first"})},
		{ID: uuid.New().String(), Type: EventPostPublished, Data: MarshalData(PostPublished{ID: "p1", URL: "https://example.com"})},
		{ID: uuid.New().String(), Type: EventPostSyndicated, Data: MarshalData(PostSyndicated{PostID: "p1", Platform: Bluesky})},
	}
	if err := store.Append(ctx, stream, events); err != nil {
		t.Fatalf("Append: %v", err)
	}

	loaded, err := store.LoadFrom(ctx, stream, 1)
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}

	if len(loaded) != 2 {
		t.Fatalf("got %d events, want 2 (after sequence 1)", len(loaded))
	}
	if loaded[0].Sequence != 2 {
		t.Errorf("first event sequence = %d, want 2", loaded[0].Sequence)
	}
	if loaded[1].Sequence != 3 {
		t.Errorf("second event sequence = %d, want 3", loaded[1].Sequence)
	}
}

func TestPostgresEventStore_LoadEmpty(t *testing.T) {
	skipIfNoPostgres(t)
	store := openTestEventStore(t)
	ctx := context.Background()

	stream := "post-" + uuid.New().String()
	loaded, err := store.Load(ctx, stream)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(loaded) != 0 {
		t.Fatalf("got %d events, want 0", len(loaded))
	}
}

func TestPostgresEventStore_StreamIsolation(t *testing.T) {
	skipIfNoPostgres(t)
	store := openTestEventStore(t)
	ctx := context.Background()

	stream1 := "post-" + uuid.New().String()
	stream2 := "post-" + uuid.New().String()

	store.Append(ctx, stream1, []fact.Event{
		{ID: uuid.New().String(), Type: EventPostCreated, Data: MarshalData(PostCreated{ID: "p1", Kind: Short, Body: "stream1"})},
	})
	store.Append(ctx, stream2, []fact.Event{
		{ID: uuid.New().String(), Type: EventPostCreated, Data: MarshalData(PostCreated{ID: "p2", Kind: Short, Body: "stream2"})},
	})

	loaded1, _ := store.Load(ctx, stream1)
	loaded2, _ := store.Load(ctx, stream2)

	if len(loaded1) != 1 {
		t.Errorf("stream1: got %d events, want 1", len(loaded1))
	}
	if len(loaded2) != 1 {
		t.Errorf("stream2: got %d events, want 1", len(loaded2))
	}

	// Both streams should have sequence 1
	if loaded1[0].Sequence != 1 {
		t.Errorf("stream1 sequence = %d, want 1", loaded1[0].Sequence)
	}
	if loaded2[0].Sequence != 1 {
		t.Errorf("stream2 sequence = %d, want 1", loaded2[0].Sequence)
	}
}

func TestPostgresEventStore_Metadata(t *testing.T) {
	skipIfNoPostgres(t)
	store := openTestEventStore(t)
	ctx := context.Background()

	stream := "post-" + uuid.New().String()
	events := []fact.Event{
		{
			ID:       uuid.New().String(),
			Type:     EventPostCreated,
			Data:     MarshalData(PostCreated{ID: "p1", Kind: Short, Body: "with metadata"}),
			Metadata: map[string]string{"user": "ben", "source": "cli"},
		},
	}

	if err := store.Append(ctx, stream, events); err != nil {
		t.Fatalf("Append: %v", err)
	}

	loaded, err := store.Load(ctx, stream)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if loaded[0].Metadata["user"] != "ben" {
		t.Errorf("Metadata[user] = %q, want %q", loaded[0].Metadata["user"], "ben")
	}
	if loaded[0].Metadata["source"] != "cli" {
		t.Errorf("Metadata[source] = %q, want %q", loaded[0].Metadata["source"], "cli")
	}
}

func TestPostgresEventStore_LoadAll(t *testing.T) {
	skipIfNoPostgres(t)
	store := openTestEventStore(t)
	ctx := context.Background()

	// Create events across multiple streams
	store.Append(ctx, "post-aaa", []fact.Event{
		{ID: uuid.New().String(), Type: EventPostCreated, Data: MarshalData(PostCreated{ID: "aaa", Kind: Short, Body: "first"})},
	})
	store.Append(ctx, "post-bbb", []fact.Event{
		{ID: uuid.New().String(), Type: EventPostCreated, Data: MarshalData(PostCreated{ID: "bbb", Kind: Long, Body: "second"})},
	})
	store.Append(ctx, "post-aaa", []fact.Event{
		{ID: uuid.New().String(), Type: EventPostPublished, Data: MarshalData(PostPublished{ID: "aaa", URL: "https://example.com/aaa"})},
	})

	all, err := store.LoadAll(ctx)
	if err != nil {
		t.Fatalf("LoadAll: %v", err)
	}

	if len(all) != 3 {
		t.Fatalf("got %d events, want 3", len(all))
	}

	// Should be ordered by occurred_at, sequence
	for i := 1; i < len(all); i++ {
		if all[i].OccurredAt.Before(all[i-1].OccurredAt) {
			t.Errorf("event[%d] occurred before event[%d]", i, i-1)
		}
	}
}

func TestPostgresEventStore_ReplayIntoProjection(t *testing.T) {
	skipIfNoPostgres(t)
	ctx := context.Background()

	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		dsn = "postgres://localhost:5432/lamina?sslmode=disable"
	}
	db := axon.OpenTestDB(t, dsn, Migrations)

	// Create store with projection, write some events
	projection1 := &PostProjection{}
	store1 := NewPostgresEventStore(db, WithPgProjector(projection1))

	store1.Append(ctx, "post-x1", []fact.Event{
		{ID: uuid.New().String(), Type: EventPostCreated, Data: MarshalData(PostCreated{ID: "x1", Kind: Short, Body: "persisted post"})},
	})
	store1.Append(ctx, "post-x1", []fact.Event{
		{ID: uuid.New().String(), Type: EventPostPublished, Data: MarshalData(PostPublished{ID: "x1", URL: "https://example.com/x1", PublishedAt: time.Now().UTC()})},
	})

	// Simulate a new process: fresh projection, replay from DB
	projection2 := &PostProjection{}
	store2 := NewPostgresEventStore(db, WithPgProjector(projection2))

	if err := store2.Replay(ctx); err != nil {
		t.Fatalf("Replay: %v", err)
	}

	post := projection2.Get("x1")
	if post == nil {
		t.Fatal("post x1 not found after replay")
	}
	if post.Body != "persisted post" {
		t.Errorf("Body = %q", post.Body)
	}
	if post.PublishedAt.IsZero() {
		t.Error("expected non-zero PublishedAt after replay")
	}
}

// Verify PostgresEventStore satisfies the EventStore interface at compile time.
var _ fact.EventStore = (*PostgresEventStore)(nil)
