// Package branch implements `hv branch <deck> <branch>` and the shared
// branching logic used by `hv init` after provisioning.
package branch

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/git"
	"github.com/talkersoft/hive-deck/internal/resolve"
)

// Run is the full standalone hv branch command.
// All repos must be on disk, on the default branch, clean, and the branch
// must not already exist in any repo.
func Run(l *config.Loaded, branchName string) error {
	if err := l.ValidateDeck(); err != nil {
		return err
	}

	plan, err := resolve.Build(l)
	if err != nil {
		return err
	}

	// All repos must be on disk.
	var missing []string
	for _, r := range plan.Repos {
		if !isGitRepo(r.Dest) {
			missing = append(missing, r.Repo)
		}
	}
	if len(missing) > 0 {
		fmt.Fprintln(os.Stderr, "error: repos not on disk — run hv init first:")
		for _, m := range missing {
			fmt.Fprintf(os.Stderr, "  %s\n", m)
		}
		return fmt.Errorf("all repos must be provisioned before branching")
	}

	if err := preFlightRepos(plan.Repos, branchName); err != nil {
		return err
	}

	return createAll(plan.Repos, branchName, plan.Deck)
}

// PreFlightExisting checks only repos already on disk — used by init before
// provisioning to fail fast if any existing repo is not ready to branch.
func PreFlightExisting(l *config.Loaded, branchName string) error {
	if err := l.ValidateDeck(); err != nil {
		return err
	}
	plan, err := resolve.Build(l)
	if err != nil {
		return err
	}
	var existing []resolve.RepoPlan
	for _, r := range plan.Repos {
		if isGitRepo(r.Dest) {
			existing = append(existing, r)
		}
	}
	return preFlightRepos(existing, branchName)
}

// CreateAll creates the branch in every repo in the plan.
// Used by init after provision completes — no pre-flight, assumes clean state.
func CreateAll(l *config.Loaded, branchName string) error {
	plan, err := resolve.Build(l)
	if err != nil {
		return err
	}
	var repos []resolve.RepoPlan
	for _, r := range plan.Repos {
		if isGitRepo(r.Dest) {
			repos = append(repos, r)
		}
	}
	return createAll(repos, branchName, plan.Deck)
}

// preFlightRepos checks a set of repos: on default branch, clean, and branch
// does not exist. Accumulates all problems before returning.
func preFlightRepos(repos []resolve.RepoPlan, branchName string) error {
	type problem struct {
		repo   string
		reason string
	}
	var problems []problem

	for _, r := range repos {
		def := defaultBranch(r.Dest)
		current, err := currentBranch(r.Dest)
		if err != nil {
			problems = append(problems, problem{r.Repo, fmt.Sprintf("could not read branch: %v", err)})
			continue
		}
		if current != def {
			problems = append(problems, problem{r.Repo, fmt.Sprintf("on %q, not default branch %q — run hv default first", current, def)})
			continue
		}
		st := git.Check(r.Dest, r.Repo)
		if !st.Clean {
			for _, reason := range st.Reasons {
				problems = append(problems, problem{r.Repo, reason})
			}
			continue
		}
		if branchExists(r.Dest, branchName) {
			problems = append(problems, problem{r.Repo, fmt.Sprintf("branch %q already exists", branchName)})
		}
	}

	if len(problems) > 0 {
		fmt.Fprintln(os.Stderr, "error: cannot branch:")
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", p.repo, p.reason)
		}
		return fmt.Errorf("resolve the above issues before branching")
	}
	return nil
}

func createAll(repos []resolve.RepoPlan, branchName, deck string) error {
	fmt.Printf("branch: %s → %s repos=%d\n", deck, branchName, len(repos))
	for _, r := range repos {
		if err := checkoutNewBranch(r.Dest, branchName); err != nil {
			return fmt.Errorf("%s: checkout -b %s: %w", r.Repo, branchName, err)
		}
		fmt.Printf("  branch  %-30s %s\n", r.Repo, branchName)
	}
	return nil
}

func branchExists(dir, branchName string) bool {
	cmd := exec.Command("git", "rev-parse", "--verify", branchName)
	cmd.Dir = dir
	return cmd.Run() == nil
}

func checkoutNewBranch(dir, branchName string) error {
	cmd := exec.Command("git", "checkout", "-b", branchName)
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
