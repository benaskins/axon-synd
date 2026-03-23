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

### Domain package

- `doc.go` — package documentation
- `synd.go` — Post, event types, Platform constants
- `store.go` — PostStore, PostProjection (event-sourced read model)
- `postgres_store.go` — PostgresEventStore (persistent fact.EventStore)
- `migrations.go` — embedded SQL migrations (`migrations/*.sql`) via `embed.FS`
- `site.go` — SiteBuilder
- `bluesky.go` / `mastodon.go` / `threads.go` — platform clients
- `cloudflare.go` — Cloudflare Pages deploy
- `git.go` — git commit and push
- `markdown.go` — markdown link extraction

### CLI (`cmd/synd`)

- `cmd/synd/main.go` — CLI entry point and root Cobra command
- `cmd/synd/serve.go` — `synd serve` command, starts HTTP server and background worker
- `cmd/synd/api.go` — HTTP API handler (create, approve, delete, list posts)
- `cmd/synd/web.go` — web UI handler for draft review pages (uses embedded HTML templates)
- `cmd/synd/worker.go` — background publish and syndication worker
- `cmd/synd/notify.go` — Signal notifications via axon-gate
- `cmd/synd/auth.go` — CLI authentication (token provisioning and authed HTTP requests)
- `cmd/synd/post.go` — `synd post` command (create a draft)
- `cmd/synd/posts.go` — `synd posts` command (list recent posts)
- `cmd/synd/drafts.go` — `synd drafts` command (list posts awaiting approval)
- `cmd/synd/approve.go` — `synd approve` command (approve a draft)
- `cmd/synd/revise.go` — `synd revise` command (edit a post)
- `cmd/synd/delete.go` — `synd delete` command (remove a post)
- `cmd/synd/synd.go` — `synd synd` command (syndicate to platforms)

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
