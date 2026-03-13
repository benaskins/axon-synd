package synd

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGitPublish(t *testing.T) {
	// Create a bare remote repo
	remote := t.TempDir()
	if err := git(remote, "init", "--bare"); err != nil {
		t.Fatalf("init bare: %v", err)
	}

	// Clone it as our working repo
	dir := t.TempDir()
	if err := git(dir, "clone", remote, "site"); err != nil {
		t.Fatalf("clone: %v", err)
	}
	siteDir := filepath.Join(dir, "site")

	// Create initial commit so we have a branch
	if err := os.WriteFile(filepath.Join(siteDir, "CNAME"), []byte("test.example.com"), 0o644); err != nil {
		t.Fatal(err)
	}
	git(siteDir, "add", "-A")
	git(siteDir, "commit", "-m", "initial")
	git(siteDir, "push", "-u", "origin", "main")

	// Add a file and publish
	if err := os.WriteFile(filepath.Join(siteDir, "index.html"), []byte("<h1>hello</h1>"), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := GitPublish(siteDir, "add index")
	if err != nil {
		t.Fatalf("GitPublish: %v", err)
	}
	if !changed {
		t.Error("expected changes to be committed")
	}

	// Verify commit exists
	out, _ := gitOutput(siteDir, "log", "--oneline", "-1")
	if out == "" {
		t.Error("expected commit in log")
	}
}

func TestGitPublish_NoChanges(t *testing.T) {
	remote := t.TempDir()
	git(remote, "init", "--bare")

	dir := t.TempDir()
	git(dir, "clone", remote, "site")
	siteDir := filepath.Join(dir, "site")

	os.WriteFile(filepath.Join(siteDir, "CNAME"), []byte("test.example.com"), 0o644)
	git(siteDir, "add", "-A")
	git(siteDir, "commit", "-m", "initial")
	git(siteDir, "push", "-u", "origin", "main")

	// No new changes
	changed, err := GitPublish(siteDir, "nothing")
	if err != nil {
		t.Fatalf("GitPublish: %v", err)
	}
	if changed {
		t.Error("expected no changes")
	}
}
