// Package checkout implements `hv deck default` — reset every repo in the
// deck to its default branch, after verifying all repos are fully clean.
package checkout

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/git"
	"github.com/talkersoft/hive-deck/internal/resolve"
)

type Options struct {
	Filter string
}

// Run executes the default-branch reset workflow:
//  1. Run git.Check on every repo — accumulate all problems, fail if any.
//  2. Switch every repo to its default branch and pull.
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
		defaultBranch string
	}

	var repos []repoMeta
	for _, r := range plan.Repos {
		if !isGitRepo(r.Dest) {
			continue
		}
		repos = append(repos, repoMeta{r, defaultBranch(r.Dest)})
	}

	// Phase 1: verify all repos are clean — same check as sync/teardown/pr.
	type problem struct {
		repo    string
		reasons []string
	}
	var problems []problem
	for _, rm := range repos {
		st := git.Check(rm.r.Dest, rm.r.Repo)
		if !st.Clean {
			problems = append(problems, problem{rm.r.Repo, st.Reasons})
		}
	}
	if len(problems) > 0 {
		fmt.Fprintln(os.Stderr, "error: the following repos are not clean:")
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "  %s\n", p.repo)
			for _, r := range p.reasons {
				fmt.Fprintf(os.Stderr, "    - %s\n", r)
			}
		}
		return fmt.Errorf("all repos must be clean and fully pushed before switching to default branches")
	}

	// Phase 2: switch every repo to its default branch and pull.
	fmt.Printf("default: %s repos=%d\n", plan.Deck, len(repos))
	for _, rm := range repos {
		current, err := currentBranch(rm.r.Dest)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  error %-30s could not read branch: %v\n", rm.r.Repo, err)
			continue
		}

		if current != rm.defaultBranch {
			if err := gitCheckout(rm.r.Dest, rm.defaultBranch); err != nil {
				fmt.Fprintf(os.Stderr, "  error %-30s checkout %s: %v\n", rm.r.Repo, rm.defaultBranch, err)
				continue
			}
		}

		if err := gitPull(rm.r.Dest); err != nil {
			fmt.Fprintf(os.Stderr, "  error %-30s pull: %v\n", rm.r.Repo, err)
			continue
		}

		if current == rm.defaultBranch {
			fmt.Printf("  pull  %-30s already on %s\n", rm.r.Repo, rm.defaultBranch)
		} else {
			fmt.Printf("  reset %-30s %s → %s\n", rm.r.Repo, current, rm.defaultBranch)
		}
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

func gitCheckout(dir, branch string) error {
	cmd := exec.Command("git", "checkout", branch)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func gitPull(dir string) error {
	cmd := exec.Command("git", "pull")
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
