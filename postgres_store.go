package synd

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	fact "github.com/benaskins/axon-fact"
)

// PostgresEventStore implements fact.EventStore backed by PostgreSQL.
// The caller is responsible for opening the database (via axon.OpenDB)
// and running migrations (via axon.RunMigrations) before constructing the store.
type PostgresEventStore struct {
	db             *sql.DB
	projectors     []fact.Projector
	publishers     []fact.Publisher
	onPublishError func(error)
}

// PostgresEventStoreOption configures a PostgresEventStore.
type PostgresEventStoreOption func(*PostgresEventStore)

// WithProjector registers a projector that runs synchronously on Append.
func WithPgProjector(p fact.Projector) PostgresEventStoreOption {
	return func(s *PostgresEventStore) {
		s.projectors = append(s.projectors, p)
	}
}

// WithPublisher registers a publisher that runs asynchronously after Append.
func WithPgPublisher(p fact.Publisher) PostgresEventStoreOption {
	return func(s *PostgresEventStore) {
		s.publishers = append(s.publishers, p)
	}
}

// WithPgPublishErrorHandler registers a callback invoked when a publisher
// returns an error. The callback runs in the publisher goroutine.
func WithPgPublishErrorHandler(fn func(error)) PostgresEventStoreOption {
	return func(s *PostgresEventStore) {
		s.onPublishError = fn
	}
}

// NewPostgresEventStore wraps an existing database connection as a PostgresEventStore.
func NewPostgresEventStore(db *sql.DB, opts ...PostgresEventStoreOption) *PostgresEventStore {
	s := &PostgresEventStore{db: db}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Append persists events to a stream, assigns sequence numbers atomically,
// runs projectors synchronously, then publishes asynchronously.
func (s *PostgresEventStore) Append(ctx context.Context, stream string, events []fact.Event) error {
	if len(events) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	// Get the next sequence number for this stream atomically
	var maxSeq sql.NullInt64
	err = tx.QueryRowContext(ctx, `
		SELECT MAX(sequence) FROM events WHERE stream = $1
	`, stream).Scan(&maxSeq)
	if err != nil {
		return fmt.Errorf("query max sequence: %w", err)
	}

	nextSeq := int64(1)
	if maxSeq.Valid {
		nextSeq = maxSeq.Int64 + 1
	}

	now := time.Now().UTC()
	prepared := make([]fact.Event, len(events))

	for i, e := range events {
		e.Stream = stream
		e.Sequence = nextSeq + int64(i)
		if e.OccurredAt.IsZero() {
			e.OccurredAt = now
		}

		metadataJSON, err := json.Marshal(e.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
		if e.Metadata == nil {
			metadataJSON = []byte("{}")
		}

		dataJSON := e.Data
		if dataJSON == nil {
			dataJSON = json.RawMessage("{}")
		}

		_, err = tx.ExecContext(ctx, `
			INSERT INTO events (id, stream, type, data, metadata, sequence, occurred_at)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, e.ID, e.Stream, e.Type, dataJSON, metadataJSON, e.Sequence, e.OccurredAt)
		if err != nil {
			return fmt.Errorf("insert event: %w", err)
		}

		prepared[i] = e
	}

	// Run projectors synchronously before committing
	for _, p := range s.projectors {
		for _, e := range prepared {
			if err := p.Handle(ctx, e); err != nil {
				return fmt.Errorf("projector: %w", err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tx: %w", err)
	}

	// Publish asynchronously
	if len(s.publishers) > 0 {
		go func() {
			for _, pub := range s.publishers {
				if err := pub.Publish(ctx, prepared); err != nil {
					slog.Error("publisher failed", "stream", stream, "error", err)
					if s.onPublishError != nil {
						s.onPublishError(err)
					}
				}
			}
		}()
	}

	return nil
}

// Load returns all events for a stream in sequence order.
func (s *PostgresEventStore) Load(ctx context.Context, stream string) ([]fact.Event, error) {
	return s.queryEvents(ctx, `
		SELECT id, stream, type, data, metadata, sequence, occurred_at
		FROM events
		WHERE stream = $1
		ORDER BY sequence ASC
	`, stream)
}

// LoadFrom returns events for a stream with sequence greater than fromSequence.
func (s *PostgresEventStore) LoadFrom(ctx context.Context, stream string, fromSequence int64) ([]fact.Event, error) {
	return s.queryEvents(ctx, `
		SELECT id, stream, type, data, metadata, sequence, occurred_at
		FROM events
		WHERE stream = $1 AND sequence > $2
		ORDER BY sequence ASC
	`, stream, fromSequence)
}

func (s *PostgresEventStore) queryEvents(ctx context.Context, query string, args ...any) ([]fact.Event, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []fact.Event
	for rows.Next() {
		var e fact.Event
		var metadataJSON []byte
		if err := rows.Scan(&e.ID, &e.Stream, &e.Type, &e.Data, &metadataJSON, &e.Sequence, &e.OccurredAt); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		if len(metadataJSON) > 0 && string(metadataJSON) != "{}" {
			if err := json.Unmarshal(metadataJSON, &e.Metadata); err != nil {
				return nil, fmt.Errorf("unmarshal metadata: %w", err)
			}
		}
		events = append(events, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return events, nil
}
