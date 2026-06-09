// Package ship implements `hv ship <deck> <message> --title <title>` —
// stage, commit, push, and open pull requests for every repo in the deck.
package ship

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
	Message             string
	Title               string
	Body                string
	DeleteBranchOnMerge bool
	AutoMerge           bool
}

func Run(l *config.Loaded, opts Options) error {
	if err := l.ValidateDeck(); err != nil {
		return err
	}

	plan, err := resolve.Build(l)
	if err != nil {
		return err
	}

	type repoMeta struct {
		r            resolve.RepoPlan
		branch       string
		targetBranch string
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
		tb := plan.Branch
		if tb == "" {
			tb = defaultBranch(r.Dest)
		}
		repos = append(repos, repoMeta{r, branch, tb})
	}

	if len(repos) == 0 {
		return fmt.Errorf("no repos on disk — run hv init first")
	}

	// Pre-flight: refuse to ship from default branch or onto a branch that
	// already has a PR (open or merged). Once a PR exists, the branch is done —
	// merge it and run hv_next to start fresh.
	type problem struct {
		repo   string
		reason string
	}
	var problems []problem
	for _, rm := range repos {
		if rm.branch == rm.targetBranch {
			problems = append(problems, problem{rm.r.Repo, fmt.Sprintf("on default branch %q — create a feature branch before shipping", rm.targetBranch)})
			continue
		}
		if rm.r.GitHubSlug == "" {
			continue
		}
		info := github.GetPRInfo(rm.r.Dest, rm.branch)
		switch info.State {
		case "OPEN":
			problems = append(problems, problem{rm.r.Repo, fmt.Sprintf("PR already open: %s — merge it, then run hv_next to start fresh", info.URL)})
		case "MERGED":
			problems = append(problems, problem{rm.r.Repo, fmt.Sprintf("PR already merged: %s — run hv_next to start fresh", info.URL)})
		case "CLOSED":
			problems = append(problems, problem{rm.r.Repo, fmt.Sprintf("PR was closed: %s — run hv_next to open a new branch", info.URL)})
		}
	}
	if len(problems) > 0 {
		fmt.Fprintln(os.Stderr, "error: cannot ship:")
		for _, p := range problems {
			fmt.Fprintf(os.Stderr, "  %s: %s\n", p.repo, p.reason)
		}
		return fmt.Errorf("resolve the above issues before shipping")
	}

	// Phase 1: commit and push repos with changes.
	fmt.Printf("ship: %s repos=%d\n", plan.Deck, len(repos))
	for _, rm := range repos {
		dirty := hasUncommitted(rm.r.Dest)
		unpushed := hasUnpushed(rm.r.Dest)

		if !dirty && !unpushed {
			fmt.Printf("  skip    %-30s nothing to ship\n", rm.r.Repo)
			continue
		}

		if dirty {
			if err := runGitCmd(rm.r.Dest, "git", "add", "-A"); err != nil {
				return fmt.Errorf("%s: git add: %w", rm.r.Repo, err)
			}
			if err := runGitCmd(rm.r.Dest, "git", "commit", "-m", opts.Message); err != nil {
				return fmt.Errorf("%s: git commit: %w", rm.r.Repo, err)
			}
		}

		if err := gitPush(rm.r.Dest, rm.branch); err != nil {
			return fmt.Errorf("%s: git push: %w", rm.r.Repo, err)
		}

		fmt.Printf("  ship    %-30s %s\n", rm.r.Repo, opts.Message)
	}

	// Phase 2: open PRs for repos on a feature branch ahead of origin/<default>.
	fmt.Println()
	var created []string
	for _, rm := range repos {
		if commitsAhead(rm.r.Dest, "origin/"+rm.targetBranch) == 0 {
			continue
		}
		if rm.r.GitHubSlug == "" {
			fmt.Printf("  skip    %-30s no GitHub remote\n", rm.r.Repo)
			continue
		}
		url, err := createPR(rm.r.Dest, rm.targetBranch, opts.Title, opts.Body)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  error   %-30s %v\n", rm.r.Repo, err)
			continue
		}
		if opts.DeleteBranchOnMerge {
			if err := setDeleteBranchOnMerge(rm.r.GitHubSlug); err != nil {
				fmt.Fprintf(os.Stderr, "  warn    %-30s delete-branch-on-merge: %v\n", rm.r.Repo, err)
			}
		}
		fmt.Printf("  pr      %-30s %s\n", rm.r.Repo, url)
		created = append(created, url)

		if opts.AutoMerge {
			if err := mergePR(rm.r.Dest); err != nil {
				fmt.Fprintf(os.Stderr, "  warn    %-30s auto-merge: %v\n", rm.r.Repo, err)
			} else {
				fmt.Printf("  merged  %-30s %s\n", rm.r.Repo, url)
			}
		}
	}

	if len(created) == 0 {
		fmt.Println("no pull requests created")
		return nil
	}

	fmt.Printf("\ncreated %d pull request(s):\n", len(created))
	for _, u := range created {
		fmt.Println(" ", u)
	}
	if opts.AutoMerge {
		fmt.Println("\nauto-merged: all pull requests merged")
	}
	return nil
}

func mergePR(dir string) error {
	cmd := exec.Command("gh", "pr", "merge", "--merge", "--delete-branch=false")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh pr merge: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func hasUncommitted(dir string) bool {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(out))) > 0
}

func hasUnpushed(dir string) bool {
	cmd := exec.Command("git", "rev-list", "--count", "@{u}..HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return false
	}
	n := 0
	fmt.Sscanf(strings.TrimSpace(string(out)), "%d", &n)
	return n > 0
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

func setDeleteBranchOnMerge(slug string) error {
	cmd := exec.Command("gh", "repo", "edit", slug, "--delete-branch-on-merge")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("gh repo edit: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

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

func gitPush(dir, branch string) error {
	cmd := exec.Command("git", "push", "--set-upstream", "origin", branch)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s", strings.TrimSpace(string(out)))
	}
	return nil
}

func runGitCmd(dir, name string, args ...string) error {
	cmd := exec.Command(name, args...)
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
