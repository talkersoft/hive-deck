// Package provision implements `hv provision <deck>`.
// Provision is idempotent: missing repos are cloned, shell dirs (no .git/) are
// restored in place, and already-cloned repos are skipped. Run it any number
// of times — the result is always a fully provisioned deck.
//
// GitHub create-if-missing is always on: for each repo that doesn't yet exist
// on github.com, `gh repo create --private --add-readme` runs before the clone.
// The `gh` CLI is a hard runtime dependency and every in-scope org must resolve
// to a github.com host; both are checked upfront.
//
// Every repo successfully cloned during a run is tracked; if any subsequent
// step fails, every tracked clone from this run is removed before the error
// returns. Intermediate folders are NOT removed. GitHub repo creations are NOT
// rolled back.
package provision

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/talkersoft/hive-deck/internal/claude"
	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/git"
	gh "github.com/talkersoft/hive-deck/internal/github"
	"github.com/talkersoft/hive-deck/internal/resolve"
	"github.com/talkersoft/hive-deck/internal/workspace"
)

type Options struct {
	Out io.Writer
}

func Run(l *config.Loaded, opts Options) error {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}

	if err := l.ValidateDeck(); err != nil {
		return err
	}

	plan, err := resolve.Build(l)
	if err != nil {
		return err
	}

	if err := gh.EnsureAvailable(); err != nil {
		return err
	}
	var nonGitHub []string
	for _, r := range plan.Repos {
		if r.GitHubSlug == "" {
			nonGitHub = append(nonGitHub, fmt.Sprintf("%s (%s)", r.Repo, r.URL))
		}
	}
	if len(nonGitHub) > 0 {
		return fmt.Errorf("hv deck provision only supports github.com orgs; the following repos resolve to other hosts:\n  %s", strings.Join(nonGitHub, "\n  "))
	}

	return runProvision(plan, l, opts)
}

func runProvision(plan *resolve.Plan, l *config.Loaded, opts Options) (err error) {
	var missing, restore []resolve.RepoPlan
	for _, r := range plan.Repos {
		ex, err := dirExists(r.Dest)
		if err != nil {
			return err
		}
		if !ex {
			missing = append(missing, r)
			continue
		}
		gitEx, err := dirExists(filepath.Join(r.Dest, ".git"))
		if err != nil {
			return err
		}
		if !gitEx {
			restore = append(restore, r)
		}
	}

	fmt.Fprintf(opts.Out, "provision: %s missing=%d restore=%d total=%d\n", plan.Deck, len(missing), len(restore), len(plan.Repos))

	// Always run workspace-level setup (idempotent) even if no repos are missing.
	if err = ensureFolders(plan, l.Setup); err != nil {
		return err
	}
	if wp := l.Setup.Workspace.Profile; wp != "" {
		if cs, ok := l.ClaudeProfiles[wp]; ok {
			if err = claude.MaybeWrite(filepath.Dir(plan.DeckDir), cs); err != nil {
				return err
			}
		}
	}
	if dp := l.DeckFile.ClaudeProfile; dp != "" {
		if cs, ok := l.ClaudeProfiles[dp]; ok {
			if err = claude.MaybeWrite(plan.DeckDir, cs); err != nil {
				return err
			}
		}
	}
	if err = applySymlinks(plan, opts.Out); err != nil {
		return err
	}

	if len(missing) == 0 && len(restore) == 0 {
		if err = workspace.Regenerate(plan, l); err != nil {
			return err
		}
		fmt.Fprintln(opts.Out, "regenerated .code-workspace file")
		return nil
	}

	tx := &tx{out: opts.Out}
	defer func() {
		if err != nil {
			tx.rollback()
		}
	}()
	// Restorations first (not in rollback set — idempotent additive ops).
	for _, r := range restore {
		if err = restoreRepo(r, opts.Out, l.Setup); err != nil {
			return err
		}
	}

	if err = cloneRepos(missing, opts.Out, tx, l.Setup); err != nil {
		return err
	}
	if err = workspace.Regenerate(plan, l); err != nil {
		return err
	}
	fmt.Fprintln(opts.Out, "regenerated .code-workspace file")
	return nil
}

// ensureFolders creates and scaffolds every directory in plan.Nodes.
func ensureFolders(plan *resolve.Plan, setup config.Setup) error {
	for _, node := range plan.Nodes {
		if err := os.MkdirAll(node.Dir, 0o755); err != nil {
			return err
		}
		if err := applyScaffold(node.Dir, filepath.Base(node.Dir), node.Ruleset, setup); err != nil {
			return err
		}
	}
	return nil
}

// applySymlinks creates symlinks declared in plan.Symlinks.
// Idempotent: skips symlinks that already point to the correct target.
// Errors if the dest path exists but is not a symlink, or points elsewhere.
func applySymlinks(plan *resolve.Plan, out io.Writer) error {
	for _, sl := range plan.Symlinks {
		fi, err := os.Lstat(sl.Dest)
		if err == nil {
			if fi.Mode()&os.ModeSymlink != 0 {
				current, readErr := os.Readlink(sl.Dest)
				if readErr != nil {
					return readErr
				}
				if current == sl.Target {
					continue // already correct
				}
				return fmt.Errorf("symlink %s already exists pointing to %q, expected %q", sl.Dest, current, sl.Target)
			}
			return fmt.Errorf("%s exists and is not a symlink", sl.Dest)
		}
		if !os.IsNotExist(err) {
			return err
		}
		fmt.Fprintf(out, "  symlink %s -> %s\n", sl.Dest, sl.Target)
		if err := os.Symlink(sl.Target, sl.Dest); err != nil {
			return err
		}
	}
	return nil
}

// restoreRepo brings a shell dir (no .git/, only preserved untracked files)
// back to a functional clone.
func restoreRepo(r resolve.RepoPlan, out io.Writer, setup config.Setup) error {
	if err := ensureGitHubRepo(r, out); err != nil {
		return err
	}
	fmt.Fprintf(out, "  [%s] restore %s @ %s into %s\n", r.NodePath, r.URL, r.Branch, r.Dest)
	if err := git.RestoreInPlace(r.Dest, r.URL, r.Branch); err != nil {
		return err
	}
	return applyRepoScaffold(r.Dest, r.GitignoreRuleset, setup)
}

// ── shared clone loop ────────────────────────────────────────────────────────

func cloneRepos(repos []resolve.RepoPlan, out io.Writer, tx *tx, setup config.Setup) error {
	for _, r := range repos {
		if err := ensureGitHubRepo(r, out); err != nil {
			return err
		}
		// Ensure the parent dir exists (git clone only creates the leaf dir).
		if err := os.MkdirAll(filepath.Dir(r.Dest), 0o755); err != nil {
			return err
		}
		cloneBranch := r.Branch
		if !remoteBranchExists(r.URL, r.Branch) {
			trueDefault := gh.DefaultBranch(r.GitHubSlug)
			fmt.Fprintf(out, "  [%s] branch %q not found on remote — cloning from true default %q\n", r.NodePath, r.Branch, trueDefault)
			cloneBranch = trueDefault
		}
		fmt.Fprintf(out, "  [%s] clone %s @ %s -> %s\n", r.NodePath, r.URL, cloneBranch, r.Dest)
		if err := git.Clone(r.URL, cloneBranch, r.Dest); err != nil {
			return err
		}
		if cloneBranch != r.Branch {
			cmd := exec.Command("git", "checkout", "-b", r.Branch)
			cmd.Dir = r.Dest
			if out2, err2 := cmd.CombinedOutput(); err2 != nil {
				return fmt.Errorf("create branch %q in %s: %s", r.Branch, r.Dest, strings.TrimSpace(string(out2)))
			}
		}
		tx.record(r.Dest)
		if err := applyRepoScaffold(r.Dest, r.GitignoreRuleset, setup); err != nil {
			return err
		}
	}
	return nil
}

func ensureGitHubRepo(r resolve.RepoPlan, out io.Writer) error {
	exists, err := gh.Exists(r.GitHubSlug)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	fmt.Fprintf(out, "  [%s] CREATE github.com/%s (private, --add-readme)\n", r.NodePath, r.GitHubSlug)
	return gh.Create(r.GitHubSlug)
}

// ── rollback ─────────────────────────────────────────────────────────────────

type tx struct {
	out    io.Writer
	cloned []string
}

func (t *tx) record(dest string) {
	t.cloned = append(t.cloned, dest)
}

func (t *tx) rollback() {
	if len(t.cloned) == 0 {
		return
	}
	fmt.Fprintf(t.out, "rollback: removing %d clone(s) from this run (intermediate folders kept):\n", len(t.cloned))
	for _, d := range t.cloned {
		fmt.Fprintf(t.out, "  rm -rf %s\n", d)
		if rmErr := os.RemoveAll(d); rmErr != nil {
			fmt.Fprintf(t.out, "    (rollback warning: %v)\n", rmErr)
		}
	}
}

// ── helpers ──────────────────────────────────────────────────────────────────

// resolveGitignoreEntries returns the gitignore entries for the given ruleset name.
// If ruleset is set, it is looked up in setup.GitignoreRulesets.
// Falls back to setup.Gitignore.Entries, then DefaultGitignoreEntries.
func resolveGitignoreEntries(setup config.Setup, ruleset string) []string {
	if ruleset != "" {
		if rs, ok := setup.GitignoreRulesets[ruleset]; ok {
			return rs.Entries
		}
	}
	if len(setup.Gitignore.Entries) > 0 {
		return setup.Gitignore.Entries
	}
	return DefaultGitignoreEntries
}

// applyRepoScaffold writes missing gitignore entries into a repo's own
// .gitignore, making the change visible in git status.
func applyRepoScaffold(repoDir, ruleset string, setup config.Setup) error {
	return EnsureGitignore(repoDir, resolveGitignoreEntries(setup, ruleset))
}

// ApplyRepoScaffold is the exported form of applyRepoScaffold, for use by
// repo_add after cloning or initializing a single new repo.
func ApplyRepoScaffold(repoDir, ruleset string, setup config.Setup) error {
	return applyRepoScaffold(repoDir, ruleset, setup)
}

func applyScaffold(dir, name, ruleset string, setup config.Setup) error {
	if err := EnsureGitignore(dir, resolveGitignoreEntries(setup, ruleset)); err != nil {
		return err
	}
	if setup.Readme.Enabled {
		return EnsureReadme(dir, name)
	}
	return nil
}

func dirExists(path string) (bool, error) {
	fi, err := os.Stat(path)
	if err == nil {
		return fi.IsDir(), nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// remoteBranchExists reports whether <branch> exists on the remote via git ls-remote.
func remoteBranchExists(url, branch string) bool {
	cmd := exec.Command("git", "ls-remote", "--heads", url, "refs/heads/"+branch)
	out, err := cmd.Output()
	return err == nil && len(strings.TrimSpace(string(out))) > 0
}
