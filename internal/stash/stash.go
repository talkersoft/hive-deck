// Package stash implements `hv stash` and `hv unstash` — a single-entry
// push/pop escape hatch for the deadlock where a branch has a merged PR
// but uncommitted changes block hv next.
package stash

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/github"
	"github.com/talkersoft/hive-deck/internal/resolve"
)

// Push stashes uncommitted changes across all repos that have them.
// Pre-flight: every dirty repo must have a merged or closed PR on its current
// branch — otherwise the repo is shippable and hv ship should be used instead.
func Push(l *config.Loaded) error {
	if err := l.ValidateDeck(); err != nil {
		return err
	}

	plan, err := resolve.Build(l)
	if err != nil {
		return err
	}

	type repoMeta struct {
		r      resolve.RepoPlan
		branch string
	}

	var repos []repoMeta
	for _, r := range plan.Repos {
		if !isGitRepo(r.Dest) {
			continue
		}
		branch, err := currentBranch(r.Dest)
		if err != nil {
			continue
		}
		repos = append(repos, repoMeta{r, branch})
	}

	// Separate dirty from clean repos.
	type dirtyRepo struct {
		repoMeta
		prState string
		prURL   string
	}
	var dirty []dirtyRepo
	for _, rm := range repos {
		if !hasUncommitted(rm.r.Dest) {
			continue
		}
		info := github.GetPRInfo(rm.r.Dest, rm.branch)
		dirty = append(dirty, dirtyRepo{rm, info.State, info.URL})
	}

	if len(dirty) == 0 {
		fmt.Println("stash: nothing to stash — all repos are clean")
		return nil
	}

	// Pre-flight: every dirty repo must have a merged or closed PR.
	type problem struct {
		repo   string
		reason string
	}
	var problems []problem
	for _, dr := range dirty {
		switch dr.prState {
		case "MERGED", "CLOSED":
			// blocked branch — stash is valid
		case "OPEN":
			problems = append(problems, problem{dr.r.Repo, fmt.Sprintf("PR is open (%s) — use hv ship instead", dr.prURL)})
		default:
			problems = append(problems, problem{dr.r.Repo, "no merged PR — use hv ship instead"})
		}
	}
	if len(problems) > 0 {
		fmt.Fprintln(os.Stderr, "error: cannot stash — these repos are shippable:")
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", p.repo, p.reason)
		}
		return fmt.Errorf("use hv ship to commit and push these changes")
	}

	// Push stash on every dirty repo.
	fmt.Printf("stash: %s repos=%d\n", plan.Deck, len(dirty))
	for _, dr := range dirty {
		if err := gitStashPush(dr.r.Dest); err != nil {
			return fmt.Errorf("%s: git stash push: %w", dr.r.Repo, err)
		}
		fmt.Printf("  stash   %-30s %s\n", dr.r.Repo, dr.branch)
	}

	return nil
}

// Pop restores stashed changes across all repos that have stash entries.
func Pop(l *config.Loaded) error {
	if err := l.ValidateDeck(); err != nil {
		return err
	}

	plan, err := resolve.Build(l)
	if err != nil {
		return err
	}

	var popped int
	fmt.Printf("unstash: %s\n", plan.Deck)
	for _, r := range plan.Repos {
		if !isGitRepo(r.Dest) {
			continue
		}
		if !hasStash(r.Dest) {
			continue
		}
		if err := gitStashPop(r.Dest); err != nil {
			return fmt.Errorf("%s: git stash pop: %w", r.Repo, err)
		}
		branch, _ := currentBranch(r.Dest)
		fmt.Printf("  pop     %-30s %s\n", r.Repo, branch)
		popped++
	}

	if popped == 0 {
		fmt.Println("  nothing to pop — no repos have stash entries")
	}

	return nil
}

func hasUncommitted(dir string) bool {
	cmd := exec.Command("git", "status", "--porcelain", "-uno")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

func hasStash(dir string) bool {
	cmd := exec.Command("git", "stash", "list")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

func gitStashPush(dir string) error {
	cmd := exec.Command("git", "stash", "push")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func gitStashPop(dir string) error {
	cmd := exec.Command("git", "stash", "pop")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func currentBranch(dir string) (string, error) {
	out, err := runGit(dir, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	b := strings.TrimSpace(out)
	if b == "HEAD" {
		return "", fmt.Errorf("detached HEAD")
	}
	return b, nil
}

func isGitRepo(dir string) bool {
	fi, err := os.Stat(dir + "/.git")
	return err == nil && (fi.IsDir() || !fi.IsDir())
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(out), nil
}
