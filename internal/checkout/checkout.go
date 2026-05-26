// Package checkout implements `hv next` — transition every repo to a new branch
// based on origin/<default>, without ever checking out the default branch locally.
package checkout

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/github"
	"github.com/talkersoft/hive-deck/internal/resolve"
)

type Options struct {
	RequireMergedPR bool
	NextBranch      string
}

// Run executes the next-branch transition workflow:
//  1. Validate NextBranch is not the default branch name.
//  2. Verify every repo is safe to transition — accumulate all problems, fail if any.
//  3. Fetch origin and checkout -b <NextBranch> origin/<default> in every repo.
//
// The local default branch is never checked out.
func Run(l *config.Loaded, opts Options) error {
	if err := l.ValidateDeck(); err != nil {
		return err
	}

	plan, err := resolve.Build(l)
	if err != nil {
		return err
	}

	type repoMeta struct {
		r             resolve.RepoPlan
		defaultBranch string
	}

	var repos []repoMeta
	for _, r := range plan.Repos {
		if !isGitRepo(r.Dest) {
			continue
		}
		repos = append(repos, repoMeta{r, defaultBranch(r.Dest)})
	}

	// Validate: NextBranch must not be the default branch.
	for _, rm := range repos {
		if opts.NextBranch == rm.defaultBranch {
			return fmt.Errorf("%q is the default branch — hv next requires a feature branch name", opts.NextBranch)
		}
	}

	// Phase 1: verify all repos are safe to transition.
	type problem struct {
		repo    string
		reasons []string
	}
	var problems []problem
	for _, rm := range repos {
		current, err := currentBranch(rm.r.Dest)
		if err != nil {
			problems = append(problems, problem{rm.r.Repo, []string{"could not read branch: " + err.Error()}})
			continue
		}

		var reasons []string

		if uncommittedChanges(rm.r.Dest) {
			reasons = append(reasons, "working tree not clean — run hv stash first if the branch has a merged PR, otherwise run hv ship")
		}

		if len(reasons) == 0 && current != rm.defaultBranch {
			ahead := commitsAhead(rm.r.Dest, "origin/"+rm.defaultBranch)
			if ahead > 0 {
				if rm.r.GitHubSlug == "" {
					reasons = append(reasons, fmt.Sprintf("%d commit(s) on branch %q with no GitHub remote — push manually before transitioning", ahead, current))
				} else {
					info := github.GetPRInfo(rm.r.Dest, current)
					switch {
					case info.State == "":
						reasons = append(reasons, fmt.Sprintf("%d commit(s) on branch %q with no pull request — run hv ship first", ahead, current))
					case info.State == "OPEN" && opts.RequireMergedPR:
						reasons = append(reasons, fmt.Sprintf("PR %s is still open — merge it before transitioning (require_merged_pr: true)", info.URL))
					}
					// MERGED always passes; OPEN passes when RequireMergedPR is false
				}
			}
		}

		if len(reasons) > 0 {
			problems = append(problems, problem{rm.r.Repo, reasons})
		}
	}
	if len(problems) > 0 {
		fmt.Fprintln(os.Stderr, "error: the following repos are not ready to transition:")
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "  %s\n", p.repo)
			for _, r := range p.reasons {
				fmt.Fprintf(os.Stderr, "    - %s\n", r)
			}
		}
		return fmt.Errorf("resolve the above issues before transitioning")
	}

	// Phase 2: fetch origin and create new branch from origin/<default>.
	fmt.Printf("next: %s → %s repos=%d\n", plan.Deck, opts.NextBranch, len(repos))
	for _, rm := range repos {
		if err := gitFetch(rm.r.Dest); err != nil {
			fmt.Fprintf(os.Stderr, "  error %-30s fetch: %v\n", rm.r.Repo, err)
			continue
		}
		remoteRef := "origin/" + rm.defaultBranch
		if err := gitCheckoutNewFrom(rm.r.Dest, opts.NextBranch, remoteRef); err != nil {
			fmt.Fprintf(os.Stderr, "  error %-30s checkout -b %s %s: %v\n", rm.r.Repo, opts.NextBranch, remoteRef, err)
			continue
		}
		fmt.Printf("  next  %-30s → %s\n", rm.r.Repo, opts.NextBranch)
	}

	return nil
}

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

func uncommittedChanges(dir string) bool {
	cmd := exec.Command("git", "status", "--porcelain", "-uno")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

func commitsAhead(dir, ref string) int {
	out, err := runGit(dir, "rev-list", "--count", ref+"..HEAD")
	if err != nil {
		return 0
	}
	n := 0
	fmt.Sscanf(strings.TrimSpace(out), "%d", &n)
	return n
}

func gitFetch(dir string) error {
	cmd := exec.Command("git", "fetch", "origin")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func gitCheckoutNewFrom(dir, branch, from string) error {
	cmd := exec.Command("git", "checkout", "-b", branch, from)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
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
