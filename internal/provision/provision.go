// Package provision implements `hv deck provision <deck> [--filter <node>]`.
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
	Filter string // node name filter (empty = all)
	Out    io.Writer
}

func Run(l *config.Loaded, opts Options) error {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}

	if err := l.ValidateDeck(); err != nil {
		return err
	}

	plan, err := resolve.Build(l, opts.Filter)
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
	if err = claude.MaybeWrite(filepath.Dir(plan.DeckDir), l.Setup.ClaudeSettings); err != nil {
		return err
	}
	if err = claude.MaybeWrite(plan.DeckDir, l.Setup.ClaudeSettings); err != nil {
		return err
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

// ensureFolders creates and scaffolds every directory in plan.Folders.
func ensureFolders(plan *resolve.Plan, setup config.Setup) error {
	for _, dir := range plan.Folders {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := applyScaffold(dir, filepath.Base(dir), setup); err != nil {
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
	if err := applyRepoScaffold(r.Dest, setup); err != nil {
		return err
	}
	return claude.MaybeWrite(r.Dest, setup.ClaudeSettings)
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
		fmt.Fprintf(out, "  [%s] clone %s @ %s -> %s\n", r.NodePath, r.URL, r.Branch, r.Dest)
		if err := git.Clone(r.URL, r.Branch, r.Dest); err != nil {
			return err
		}
		tx.record(r.Dest)
		if err := applyRepoScaffold(r.Dest, setup); err != nil {
			return err
		}
		if err := claude.MaybeWrite(r.Dest, setup.ClaudeSettings); err != nil {
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

// applyRepoScaffold writes missing gitignore entries into a repo's own
// .gitignore, making the change visible in git status.
func applyRepoScaffold(repoDir string, setup config.Setup) error {
	entries := setup.Gitignore.Entries
	if len(entries) == 0 {
		entries = DefaultGitignoreEntries
	}
	return EnsureGitignore(repoDir, entries)
}

func applyScaffold(dir, name string, setup config.Setup) error {
	entries := setup.Gitignore.Entries
	if len(entries) == 0 {
		entries = DefaultGitignoreEntries
	}
	if err := EnsureGitignore(dir, entries); err != nil {
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
