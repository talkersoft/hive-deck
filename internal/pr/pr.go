// Package pr implements `hv deck pr` — create pull requests for every repo
// in a deck whose current branch is ahead of the default branch on origin.
package pr

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/git"
	"github.com/talkersoft/hive-deck/internal/resolve"
)

// Options holds the flags for the pr command.
type Options struct {
	Filter string
	Title  string
	Body   string
}

// Run executes the pr workflow:
//  1. Run git.Check on all repos — fail fast with a full problem list if anything is wrong.
//     Being ahead of upstream on a feature branch is expected; dirty working tree or
//     being on the default branch with unpushed commits are both blocking.
//  2. Create a PR for every repo on a feature branch that is ahead of origin/<default>.
//  3. Print the URL of every PR created.
func Run(l *config.Loaded, opts Options) error {
	if err := l.ValidateDeck(); err != nil {
		return err
	}

	plan, err := resolve.Build(l, opts.Filter)
	if err != nil {
		return err
	}

	type repoMeta struct {
		r             resolve.RepoPlan
		branch        string
		defaultBranch string
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
		def := defaultBranch(r.Dest)
		repos = append(repos, repoMeta{r, branch, def})
	}

	// Phase 1: pre-flight — use git.Check (same algorithm as status/teardown/sync).
	// Accumulate all problems before failing so the user sees everything at once.
	type problem struct {
		repo    string
		reasons []string
	}
	var problems []problem

	for _, rm := range repos {
		st := git.Check(rm.r.Dest, rm.r.Repo)
		if st.Clean {
			continue
		}

		onDefault := rm.branch == rm.defaultBranch

		var blocking []string
		for _, reason := range st.Reasons {
			if !onDefault && isAheadOrNoUpstream(reason) {
				// On a feature branch, being ahead of (or not yet tracking) upstream
				// is the expected state — commits are waiting to be PR'd.
				continue
			}
			msg := reason
			if onDefault && isAheadOrNoUpstream(reason) {
				msg = reason + " — create a feature branch for this work"
			}
			blocking = append(blocking, msg)
		}

		if len(blocking) > 0 {
			problems = append(problems, problem{rm.r.Repo, blocking})
		}
	}

	if len(problems) > 0 {
		fmt.Fprintln(os.Stderr, "error: the following repos have issues that must be resolved first:")
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "  %s\n", p.repo)
			for _, r := range p.reasons {
				fmt.Fprintf(os.Stderr, "    - %s\n", r)
			}
		}
		return fmt.Errorf("fix the above issues before running `hv deck pr`")
	}

	// Phase 2: create PRs for feature branches ahead of origin/<default>.
	var created []string
	for _, rm := range repos {
		if rm.branch == rm.defaultBranch {
			continue // nothing to PR from the default branch
		}
		if commitsAhead(rm.r.Dest, "origin/"+rm.defaultBranch) == 0 {
			continue // feature branch exists but nothing new relative to default
		}
		if rm.r.GitHubSlug == "" {
			fmt.Printf("  skip %-30s (no GitHub remote)\n", rm.r.Repo)
			continue
		}

		// Skip if a PR already exists for this branch.
		if existing := existingPRURL(rm.r.Dest, rm.branch); existing != "" {
			fmt.Printf("  skip %-30s PR already open: %s\n", rm.r.Repo, existing)
			continue
		}

		url, err := createPR(rm.r.Dest, rm.defaultBranch, opts.Title, opts.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  error %-30s %v\n", rm.r.Repo, err)
			continue
		}
		fmt.Printf("  pr   %-30s %s\n", rm.r.Repo, url)
		created = append(created, url)
	}

	if len(created) == 0 {
		fmt.Println("no pull requests created")
		return nil
	}

	fmt.Printf("\ncreated %d pull request(s):\n", len(created))
	for _, u := range created {
		fmt.Println(" ", u)
	}
	return nil
}

// isAheadOrNoUpstream returns true for git.Check reasons that are expected
// on a feature branch waiting to be PR'd.
func isAheadOrNoUpstream(reason string) bool {
	return strings.Contains(reason, "not pushed to upstream") ||
		strings.Contains(reason, "no upstream branch configured")
}

// currentBranch returns the current branch name, or an error if HEAD is detached.
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

// defaultBranch resolves the default branch via origin/HEAD, falling back to "main".
func defaultBranch(dir string) string {
	out, err := runGit(dir, "symbolic-ref", "refs/remotes/origin/HEAD")
	if err == nil {
		parts := strings.Split(strings.TrimSpace(out), "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}
	return "main"
}

// commitsAhead returns how many commits the current HEAD is ahead of ref.
func commitsAhead(dir, ref string) int {
	out, err := runGit(dir, "rev-list", "--count", ref+"..HEAD")
	if err != nil {
		return 0
	}
	n := 0
	fmt.Sscanf(strings.TrimSpace(out), "%d", &n)
	return n
}

// existingPRURL returns the URL of an open PR for the given branch, or "".
func existingPRURL(dir, branch string) string {
	cmd := exec.Command("gh", "pr", "view", branch, "--json", "url", "--jq", ".url")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// createPR runs `gh pr create` and returns the URL of the new PR.
func createPR(dir, base, title, body string) (string, error) {
	args := []string{"pr", "create", "--base", base, "--title", title, "--body", body}
	cmd := exec.Command("gh", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("gh pr create: %s", strings.TrimSpace(string(out)))
	}
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	return lines[len(lines)-1], nil
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
