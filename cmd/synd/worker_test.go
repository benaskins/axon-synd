package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	synd "github.com/benaskins/axon-synd"
)

func TestPublishWorker_PicksUpApprovedPost(t *testing.T) {
	store, projection := newMemoryStore()
	ctx := context.Background()

	// Create and approve a post
	post, _ := store.Create(ctx, synd.Short, "worker test")
	store.Approve(ctx, post.ID, "test")

	// Verify it shows as approved
	approved := projection.ApprovedPosts()
	if len(approved) != 1 {
		t.Fatalf("expected 1 approved post, got %d", len(approved))
	}

	// Set up a temp site dir with git
	dir := initTestGitRepo(t)

	w := &publishWorker{
		store:      store,
		projection: projection,
		siteDir:    dir,
		baseURL:    "https://example.com",
	}

	// Run one cycle
	published := w.publishApproved(ctx)
	if published != 1 {
		t.Fatalf("published = %d, want 1", published)
	}

	// Post should now be published
	got := store.Get(post.ID)
	if got.Status != synd.StatusPublished {
		t.Errorf("Status = %q, want %q", got.Status, synd.StatusPublished)
	}

	// Should no longer appear in approved list
	approved = projection.ApprovedPosts()
	if len(approved) != 0 {
		t.Errorf("expected 0 approved posts, got %d", len(approved))
	}
}

func TestPublishWorker_SkipsDrafts(t *testing.T) {
	store, projection := newMemoryStore()
	ctx := context.Background()

	// Create a draft but don't approve it
	store.Create(ctx, synd.Short, "still a draft")

	w := &publishWorker{
		store:      store,
		projection: projection,
		siteDir:    t.TempDir(),
		baseURL:    "https://example.com",
	}

	published := w.publishApproved(ctx)
	if published != 0 {
		t.Fatalf("published = %d, want 0", published)
	}
}

func TestPublishWorker_RunStopsOnCancel(t *testing.T) {
	store, projection := newMemoryStore()
	ctx, cancel := context.WithCancel(context.Background())

	w := &publishWorker{
		store:      store,
		projection: projection,
		siteDir:    t.TempDir(),
		baseURL:    "https://example.com",
		interval:   50 * time.Millisecond,
	}

	done := make(chan struct{})
	go func() {
		w.run(ctx)
		close(done)
	}()

	// Let it tick once
	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case <-done:
		// good
	case <-time.After(2 * time.Second):
		t.Fatal("worker did not stop after context cancel")
	}
}

func initTestGitRepo(t *testing.T) string {
	t.Helper()

	// Create a bare "remote" so git push works
	remote := t.TempDir()
	gitRun(t, remote, "init", "--bare")

	// Init a local repo that pushes to the bare remote
	dir := t.TempDir()
	gitRun(t, dir, "init")
	gitRun(t, dir, "config", "user.email", "test@test.com")
	gitRun(t, dir, "config", "user.name", "Test")
	gitRun(t, dir, "remote", "add", "origin", remote)

	readme := filepath.Join(dir, "README.md")
	os.WriteFile(readme, []byte("test site"), 0644)
	gitRun(t, dir, "add", ".")
	gitRun(t, dir, "commit", "-m", "init")
	gitRun(t, dir, "push", "-u", "origin", "main")
	return dir
}

func gitRun(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
