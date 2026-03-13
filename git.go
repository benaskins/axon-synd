package synd

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// GitPublish commits and pushes changes in a site repo directory.
// Returns true if changes were committed, false if there was nothing to commit.
func GitPublish(repoDir, message string) (bool, error) {
	if err := git(repoDir, "add", "-A"); err != nil {
		return false, fmt.Errorf("git add: %w", err)
	}

	// Check if there are staged changes
	out, err := gitOutput(repoDir, "status", "--porcelain")
	if err != nil {
		return false, fmt.Errorf("git status: %w", err)
	}
	if strings.TrimSpace(out) == "" {
		return false, nil
	}

	if err := git(repoDir, "commit", "-m", message); err != nil {
		return false, fmt.Errorf("git commit: %w", err)
	}

	if err := git(repoDir, "push"); err != nil {
		return false, fmt.Errorf("git push: %w", err)
	}

	return true, nil
}

// TestGit runs a git command in the given directory. For use in tests only.
func TestGit(t interface{ Helper(); Fatalf(string, ...any) }, dir string, args ...string) {
	t.Helper()
	if err := git(dir, args...); err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
}

func git(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %s", err, stderr.String())
	}
	return nil
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("%s: %s", err, stderr.String())
	}
	return stdout.String(), nil
}
