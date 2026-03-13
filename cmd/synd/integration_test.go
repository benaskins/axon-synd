package main

import (
	"context"
	"testing"
	"time"

	synd "github.com/benaskins/axon-synd"
)

func TestFullPipeline_CreateApprovePublish(t *testing.T) {
	store, projection := newMemoryStore()
	ctx := context.Background()

	// 1. Create a draft post (simulates `synd post "hello world"`)
	post, err := store.Create(ctx, synd.Short, "hello world", synd.WithApprovalToken("secret-tok"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if post.Status != synd.StatusDraft {
		t.Fatalf("expected draft, got %s", post.Status)
	}

	// 2. Revise the draft (simulates web UI edit)
	if err := store.Revise(ctx, post.ID, "hello revised world", "", "", nil, "web"); err != nil {
		t.Fatalf("revise: %v", err)
	}
	revised := store.Get(post.ID)
	if revised.Body != "hello revised world" {
		t.Fatalf("Body = %q after revise", revised.Body)
	}

	// 3. Approve the draft (simulates web UI approve)
	if err := store.Approve(ctx, post.ID, "web"); err != nil {
		t.Fatalf("approve: %v", err)
	}

	approved := store.Get(post.ID)
	if approved.Status != synd.StatusApproved {
		t.Fatalf("expected approved, got %s", approved.Status)
	}

	// 4. Worker picks up the approved post and publishes
	dir := initTestGitRepo(t)
	w := &publishWorker{
		store:      store,
		projection: projection,
		siteDir:    dir,
		baseURL:    "https://example.com",
		interval:   50 * time.Millisecond,
	}

	workerCtx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		w.run(workerCtx)
		close(done)
	}()

	// Wait for the worker to process
	deadline := time.After(5 * time.Second)
	for {
		got := store.Get(post.ID)
		if got.Status == synd.StatusPublished {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("worker did not publish within timeout; status = %s", got.Status)
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}

	cancel()
	<-done

	// 5. Verify final state
	final := store.Get(post.ID)
	if final.Status != synd.StatusPublished {
		t.Errorf("final Status = %q, want published", final.Status)
	}
	if final.Body != "hello revised world" {
		t.Errorf("final Body = %q, want %q", final.Body, "hello revised world")
	}
	if final.PublishedAt.IsZero() {
		t.Error("PublishedAt should be set")
	}

	// No more approved posts pending
	if len(projection.ApprovedPosts()) != 0 {
		t.Error("expected 0 approved posts after publish")
	}
}
