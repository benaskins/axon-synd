package synd

import (
	"context"
	"os"
	"testing"

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

// Verify PostgresEventStore satisfies the EventStore interface at compile time.
var _ fact.EventStore = (*PostgresEventStore)(nil)
