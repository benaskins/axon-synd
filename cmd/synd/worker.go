package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	synd "github.com/benaskins/axon-synd"
)

const defaultWorkerInterval = 10 * time.Second

type publishWorker struct {
	store      *synd.PostStore
	projection *synd.PostProjection
	siteDir    string
	baseURL    string
	interval   time.Duration

	// Optional deploy config — nil means skip Cloudflare deploy.
	cloudflare *synd.CloudflareConfig

	// Optional syndication configs — nil means skip that platform.
	bluesky  *synd.BlueskyConfig
	mastodon *synd.MastodonConfig
	threads  *synd.ThreadsConfig
}

// run polls for approved posts and publishes them until the context is cancelled.
func (w *publishWorker) run(ctx context.Context) {
	interval := w.interval
	if interval == 0 {
		interval = defaultWorkerInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Run immediately on start
	w.publishApproved(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.publishApproved(ctx)
		}
	}
}

// publishApproved finds all approved posts and publishes them.
// Returns the number of posts published.
func (w *publishWorker) publishApproved(ctx context.Context) int {
	posts := w.projection.ApprovedPosts()
	if len(posts) == 0 {
		return 0
	}

	published := 0
	for _, post := range posts {
		if err := w.publishOne(ctx, &post); err != nil {
			slog.Error("publish failed", "post_id", post.ID, "error", err)
			continue
		}
		published++
	}
	return published
}

func (w *publishWorker) publishOne(ctx context.Context, post *synd.Post) error {
	// Build the static site
	builder := synd.NewSiteBuilder(synd.SiteConfig{
		Title:   "Generative Plane",
		BaseURL: w.baseURL,
		Author:  "Benjamin Askins",
	})

	// Include published posts plus the post being published (still "approved" at this point).
	allPosts := w.projection.PublishedPosts()
	allPosts = append([]synd.Post{*post}, allPosts...)
	if err := builder.Build(allPosts, w.siteDir); err != nil {
		return fmt.Errorf("build site: %w", err)
	}

	// Git commit + push
	commitMsg := fmt.Sprintf("post: %s", truncateForCommit(post.Body, 50))
	changed, err := synd.GitPublish(w.siteDir, commitMsg)
	if err != nil {
		return fmt.Errorf("git publish: %w", err)
	}

	// Deploy to Cloudflare Pages
	if w.cloudflare != nil {
		if err := synd.CloudflareDeploy(*w.cloudflare, w.siteDir); err != nil {
			return fmt.Errorf("cloudflare deploy: %w", err)
		}
	}

	// Emit post.published
	url := fmt.Sprintf("%s/posts/%s", w.baseURL, post.ID)
	if err := w.store.Publish(ctx, post.ID, url); err != nil {
		return fmt.Errorf("publish event: %w", err)
	}

	if changed {
		slog.Info("published", "post_id", post.ID, "url", url)
	} else {
		slog.Info("published (no site changes)", "post_id", post.ID)
	}

	// Syndicate to configured platforms
	post = w.store.Get(post.ID) // refresh after publish event
	w.syndicatePost(ctx, post)

	return nil
}

// rebuildSite rebuilds the static site from all posts and pushes to git.
func (w *publishWorker) rebuildSite() error {
	builder := synd.NewSiteBuilder(synd.SiteConfig{
		Title:   "Generative Plane",
		BaseURL: w.baseURL,
		Author:  "Benjamin Askins",
	})

	allPosts := w.projection.PublishedPosts()
	if err := builder.Build(allPosts, w.siteDir); err != nil {
		return fmt.Errorf("build site: %w", err)
	}

	_, err := synd.GitPublish(w.siteDir, "rebuild: site updated")
	if err != nil {
		return fmt.Errorf("git publish: %w", err)
	}

	// Deploy to Cloudflare Pages
	if w.cloudflare != nil {
		if err := synd.CloudflareDeploy(*w.cloudflare, w.siteDir); err != nil {
			return fmt.Errorf("cloudflare deploy: %w", err)
		}
	}

	slog.Info("site rebuilt", "posts", len(allPosts))
	return nil
}

func (w *publishWorker) syndicatePost(ctx context.Context, post *synd.Post) {
	if w.bluesky != nil {
		if err := syndicateToBluesky(ctx, w.store, post, w.baseURL, *w.bluesky); err != nil {
			slog.Error("bluesky syndication failed", "post_id", post.ID, "error", err)
		}
	}
	if w.mastodon != nil {
		if err := syndicateToMastodon(ctx, w.store, post, w.baseURL, *w.mastodon); err != nil {
			slog.Error("mastodon syndication failed", "post_id", post.ID, "error", err)
		}
	}
	if w.threads != nil {
		if err := syndicateToThreads(ctx, w.store, post, w.baseURL, *w.threads); err != nil {
			slog.Error("threads syndication failed", "post_id", post.ID, "error", err)
		}
	}
}
