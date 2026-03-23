# axon-synd

> Services · Part of the [lamina](https://github.com/benaskins/lamina-mono) workspace

Personal syndication engine: publish posts to a canonical static site and syndicate copies to social platforms including Bluesky, Mastodon, and Threads.

## Getting started

```bash
go get github.com/benaskins/axon-synd
```

```go
store := fact.NewMemoryStore()
ps := synd.NewPostStore(store)
store.RegisterProjector(ps.Projector())

// Create a short-form post
post, _ := ps.Create(ctx, synd.Short, "Hello from the syndication engine",
    synd.WithTags("intro", "test"),
)

// Approve and publish
ps.Approve(ctx, post.ID, "ben")
ps.Publish(ctx, post.ID, "https://example.com/posts/"+post.ID)

// Build the static site
builder := synd.NewSiteBuilder(synd.SiteConfig{
    Title:   "My Site",
    BaseURL: "https://example.com",
    Author:  "Ben",
})
builder.Build(ps.Projection().PublishedPosts(), "./public")

// Syndicate to Bluesky
bsky := synd.NewBlueskyClient(synd.BlueskyConfig{
    Handle:   "me.bsky.social",
    Password: os.Getenv("BLUESKY_APP_PASSWORD"),
})
bsky.Authenticate(ctx)
uri, _, _ := bsky.Post(ctx, post.Body)
ps.Syndicate(ctx, post.ID, synd.Bluesky, uri, synd.BlueskyPostURL("me.bsky.social", uri))
```

## Key types

- **`Post`** — canonical content record with kind (short/long/image), lifecycle status, and metadata
- **`PostStore`** — event-sourced post management with create, revise, approve, publish, syndicate, and delete operations
- **`PostProjection`** — read model built from events, queryable by status (drafts, approved, published, unsynced)
- **`PostgresEventStore`** — persistent `fact.EventStore` backed by PostgreSQL, with projector and publisher support
- **`Engagement`** — metrics (likes, reposts, replies, views) for a post on a single platform
- **`SiteBuilder`** — static site generator producing index, post pages, RSS feed, and CSS from published posts
- **`BlueskyClient`** — posts to Bluesky via the AT Protocol, with text, link, and image support
- **`MastodonClient`** — posts to Mastodon via the REST API, with media uploads
- **`ThreadsClient`** — posts to Threads via Meta's Graph API
- **`CloudflareDeploy`** — uploads a built site to Cloudflare Pages via Direct Upload API
- **`GitPublish`** — commits and pushes site changes to a git repo

## CLI (`cmd/synd`)

The `synd` binary provides commands for managing posts: `post`, `posts`, `drafts`, `revise`, `approve`, `synd`, `serve`, and `delete`.

## License

MIT
