@AGENTS.md

## Conventions
- Two layers: domain package (`synd`) for content model, CLI (`cmd/synd`) for service wiring
- Event-sourced via axon-fact  - all post state changes are events appended to streams
- Posts are immutable once published  - edits create new events, not mutations
- Platform-specific adaptation happens in syndication functions, not in the post model
- Publish worker is an internal goroutine in `cmd/synd`, not an axon-task Worker
- Signal notifications via axon-gate for draft review flow

## Constraints
- Depends on axon, axon-fact, and axon-gate  - deploy gate integration is intentional (syndication can be gated)
- Do not add direct dependencies on other axon-* service packages
- Do not merge platform-specific logic into the core Post type
- Approved posts are published automatically by the background worker  - do not add manual publish steps

## Testing
- `go test ./...` runs all tests
- `go vet ./...` for lint
- Platform client tests should mock HTTP  - do not call real Bluesky/Mastodon/Threads APIs in tests
