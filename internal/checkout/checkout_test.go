package checkout

import (
	"os/exec"
	"testing"
)

func makeRepo(t *testing.T, branches ...string) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-b", "main")
	run("config", "user.email", "test@test.com")
	run("config", "user.name", "Test")
	run("commit", "--allow-empty", "-m", "init")
	for _, b := range branches {
		if b != "main" {
			run("branch", b)
		}
	}
	return dir
}

func TestGitCheckout_SwitchesBranch(t *testing.T) {
	dir := makeRepo(t, "main", "feature")

	if err := gitCheckout(dir, "feature"); err != nil {
		t.Fatalf("gitCheckout to feature: %v", err)
	}
	got, err := currentBranch(dir)
	if err != nil {
		t.Fatalf("currentBranch: %v", err)
	}
	if got != "feature" {
		t.Errorf("expected branch %q, got %q", "feature", got)
	}

	if err := gitCheckout(dir, "main"); err != nil {
		t.Fatalf("gitCheckout back to main: %v", err)
	}
	got, err = currentBranch(dir)
	if err != nil {
		t.Fatalf("currentBranch: %v", err)
	}
	if got != "main" {
		t.Errorf("expected branch %q, got %q", "main", got)
	}
}

func TestRollback_RestoresPreviousBranch(t *testing.T) {
	repoA := makeRepo(t, "main", "feature")
	repoB := makeRepo(t, "main", "feature")

	savedBranch := map[string]string{
		repoA: "main",
		repoB: "main",
	}

	// Simulate repoA successfully switched to feature
	if err := gitCheckout(repoA, "feature"); err != nil {
		t.Fatalf("setup: switch repoA to feature: %v", err)
	}
	switched := []string{repoA}

	// Rollback in reverse order
	for i := len(switched) - 1; i >= 0; i-- {
		dest := switched[i]
		if prev, ok := savedBranch[dest]; ok {
			if err := gitCheckout(dest, prev); err != nil {
				t.Errorf("rollback %s: %v", dest, err)
			}
		}
	}

	got, err := currentBranch(repoA)
	if err != nil {
		t.Fatalf("currentBranch repoA: %v", err)
	}
	if got != "main" {
		t.Errorf("repoA: expected %q after rollback, got %q", "main", got)
	}

	// repoB untouched — should still be on main
	got, err = currentBranch(repoB)
	if err != nil {
		t.Fatalf("currentBranch repoB: %v", err)
	}
	if got != "main" {
		t.Errorf("repoB: expected %q (untouched), got %q", "main", got)
	}
}
