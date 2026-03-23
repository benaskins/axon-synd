# axon-synd

Personal syndication engine. Publish posts to a canonical static site and syndicate copies to social platforms (Bluesky, Mastodon, Threads). Engagement metrics flow back for unified dashboard display.

## Architecture

axon-synd has two layers:

1. **Domain package** (`synd`) — post model, event-sourced store, site builder, platform clients
2. **CLI** (`cmd/synd`) — Cobra-based command-line tool that wires the domain into a running service

### Domain package

- **Post model** — short-form text, long-form articles, image posts
- **Event types** — post lifecycle events (created, revised, approved, published, syndicated, engagement updated)
- **PostStore** — event-sourced via axon-fact, with a PostProjection read model
- **PostgresEventStore** — persistent fact.EventStore backed by PostgreSQL, with projector and publisher support
- **Engagement** — metrics struct (likes, reposts, replies, views) per post per platform
- **SiteBuilder** — static site generator producing HTML, RSS, and CSS from published posts
- **GitPublish / CloudflareDeploy** — deployment to git repos and Cloudflare Pages
- **Platform clients** — BlueskyClient, MastodonClient, ThreadsClient for syndication
- **Migrations** — embedded SQL migrations for PostgreSQL schema

### CLI (`cmd/synd`)

- `synd post` — create a draft post
- `synd posts` / `synd drafts` — list posts
- `synd revise` — edit a draft
- `synd approve` — approve a draft for publishing
- `synd synd` — syndicate a post to platforms
- `synd serve` — run the HTTP API with a background publish worker
- `synd delete` — remove a post

The publish worker polls for approved posts, builds the site, deploys, and syndicates — it is an internal goroutine in `cmd/synd`, not an axon-task Worker.

### Signal notifications

New drafts trigger Signal notifications via axon-gate's SignalClient, linking to a review URL with an approval token.

## Key files

- `synd.go` — Post, event types, Platform constants
- `store.go` — PostStore, PostProjection (event-sourced read model)
- `postgres_store.go` — PostgresEventStore (persistent fact.EventStore)
- `site.go` — SiteBuilder
- `bluesky.go` / `mastodon.go` / `threads.go` — platform clients
- `cloudflare.go` — Cloudflare Pages deploy
- `git.go` — git commit and push
- `markdown.go` — markdown link extraction
- `cmd/synd/main.go` — CLI entry point
- `cmd/synd/worker.go` — background publish and syndication worker
- `cmd/synd/notify.go` — Signal notifications via axon-gate

## Dependencies

- `axon` — HTTP server lifecycle, config, metrics
- `axon-fact` — event store and projector interfaces
- `axon-gate` — SignalClient for draft review notifications
- `cobra` — CLI framework
- `goldmark` — markdown rendering for site generation

## Build & Test

```bash
go test ./...       # all tests
go vet ./...        # lint
```

## Conventions

- Event-sourced: all state changes are events appended to streams
- Posts are immutable once published — edits create new events
- Platform-specific adaptation happens in syndication functions, not in the post model
