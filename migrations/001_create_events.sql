-- +goose Up
CREATE TABLE IF NOT EXISTS events (
    id          TEXT NOT NULL,
    stream      TEXT NOT NULL,
    type        TEXT NOT NULL,
    data        JSONB NOT NULL DEFAULT '{}',
    metadata    JSONB NOT NULL DEFAULT '{}',
    sequence    BIGINT NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,

    PRIMARY KEY (stream, sequence)
);

CREATE INDEX IF NOT EXISTS idx_events_stream
    ON events(stream, sequence);

-- +goose Down
DROP TABLE IF EXISTS events;
