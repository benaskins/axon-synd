# axon-synd

Personal syndication engine. Publish posts to a canonical static site and syndicate copies to social platforms (Bluesky, Mastodon, Threads). Engagement metrics flow back for unified dashboard display.

## Architecture

axon-synd is a domain package (no main). It provides:

- **Post model** — short-form text, long-form articles, image posts
- **Event types** — post lifecycle events (created, published, syndicated, engagement updated)
- **Post store** — event-sourced via axon-fact, with projectors for read models
- **Site generator** — builds static HTML from posts, commits and pushes to a git repo
- **Syndication workers** — one per platform, implementing axon-task's Worker interface
- **Engagement poller** — pulls metrics from platforms back into the system

## Dependencies

- `axon` — HTTP client utilities, config
- `axon-fact` — event store, projectors

## Commands

```bash
go test ./...       # all tests
go vet ./...        # lint
```

## Conventions

- Event-sourced: all state changes are events appended to streams
- Posts are immutable once published — edits create new events
- Platform-specific adaptation happens in syndication workers, not in the post model
