// Package teardown implements `hv deck teardown <deck> [--filter <node>]` and
// `hv deck status <deck> [--filter <node>]`.
//
// Teardown is surgical preserve mode, always: for each in-scope repo it
// deletes the git-tracked files and the .git/ folder, leaves every untracked
// file in place, prunes empty subdirectories (and the repo dir itself if it
// ends up empty). The launch/ folder and every .code-workspace file inside
// it are always preserved. There is no nuke mode — if you want a full wipe,
// use `rm -rf` directly.
//
// If ANY in-scope repo has tracked work that would be lost (uncommitted,
// unpushed, detached HEAD, stash, no upstream) the entire run aborts before
// touching anything. Untracked files are NEVER a refusal reason.
//
// Re-running teardown is safe and idempotent.
package teardown

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/git"
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

	// Remove symlinks first (always safe, target is never touched).
	for _, sl := range plan.Symlinks {
		fi, err := os.Lstat(sl.Dest)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		if fi.Mode()&os.ModeSymlink == 0 {
			fmt.Fprintf(opts.Out, "  skip %s: exists but is not a symlink\n", sl.Dest)
			continue
		}
		fmt.Fprintf(opts.Out, "  remove symlink %s\n", sl.Dest)
		if err := os.Remove(sl.Dest); err != nil {
			return err
		}
	}

	// Separate repos that still exist on disk from already-gone ones.
	var active, gone []resolve.RepoPlan
	for _, r := range plan.Repos {
		ex, err := dirExists(r.Dest)
		if err != nil {
			return err
		}
		if ex {
			active = append(active, r)
		} else {
			gone = append(gone, r)
		}
	}
	for _, r := range gone {
		fmt.Fprintf(opts.Out, "skip [%s/%s]: repo dir already gone (%s)\n", r.NodePath, r.Repo, r.Dest)
	}
	if len(active) == 0 {
		fmt.Fprintln(opts.Out, "nothing to teardown — every in-scope repo is already gone")
		return workspace.Regenerate(plan, l)
	}

	// Safety check: refuse if any still-present, still-git repo has tracked work.
	var dirty []dirtyRepo
	totalRepos := 0
	for _, r := range active {
		gitExists, err := dirExists(filepath.Join(r.Dest, ".git"))
		if err != nil {
			return err
		}
		if !gitExists {
			continue // .git already gone — surgically torn down before
		}
		totalRepos++
		st := git.CheckTracked(r.Dest, r.Repo)
		if !st.Clean {
			dirty = append(dirty, dirtyRepo{nodePath: r.NodePath, status: st})
		}
	}
	if len(dirty) > 0 {
		printDirty(opts.Out, dirty)
		return fmt.Errorf("aborting: %d repo(s) have tracked work that would be lost — commit/push or clean them first (untracked files would be preserved)", len(dirty))
	}

	if totalRepos == 0 {
		fmt.Fprintln(opts.Out, "nothing to teardown — every in-scope repo is already torn down")
		return workspace.Regenerate(plan, l)
	}

	fmt.Fprintf(opts.Out, "teardown: %s repos=%d (tracked files + .git/ removed; untracked preserved)\n", plan.Deck, totalRepos)

	for _, r := range active {
		repoExists, err := dirExists(r.Dest)
		if err != nil {
			return err
		}
		if !repoExists {
			continue
		}
		gitExists, err := dirExists(filepath.Join(r.Dest, ".git"))
		if err != nil {
			return err
		}
		if !gitExists {
			continue
		}
		removed, preserved, err := surgicalRemove(r.Dest)
		if err != nil {
			return fmt.Errorf("teardown %s: %w", r.Dest, err)
		}
		if preserved == 0 {
			fmt.Fprintf(opts.Out, "  [%s] %s: removed (%d tracked files + .git/)\n", r.NodePath, r.Repo, removed)
		} else {
			fmt.Fprintf(opts.Out, "  [%s] %s: removed %d tracked + .git/, preserved %d untracked file(s) at %s\n", r.NodePath, r.Repo, removed, preserved, r.Dest)
		}
	}

	if err := workspace.Regenerate(plan, l); err != nil {
		return err
	}
	fmt.Fprintln(opts.Out, "regenerated .code-workspace file")
	return nil
}

// Status reports per-repo git state across every in-scope node without
// changing anything.
func Status(l *config.Loaded, filter string, out io.Writer) error {
	if out == nil {
		out = os.Stdout
	}

	if err := l.ValidateDeck(); err != nil {
		return err
	}

	plan, err := resolve.Build(l, filter)
	if err != nil {
		return err
	}

	// Group repos by NodePath for display.
	type repoInNode struct {
		nodePath string
		repo     resolve.RepoPlan
	}
	byNode := map[string][]resolve.RepoPlan{}
	nodeOrder := []string{}
	seen := map[string]bool{}
	for _, r := range plan.Repos {
		if !seen[r.NodePath] {
			seen[r.NodePath] = true
			nodeOrder = append(nodeOrder, r.NodePath)
		}
		byNode[r.NodePath] = append(byNode[r.NodePath], r)
	}

	grandClean := 0
	grandTotal := 0
	for _, nodePath := range nodeOrder {
		repos := byNode[nodePath]
		display := nodePath
		if display == "" {
			display = plan.Deck
		}
		fmt.Fprintf(out, "=== %s ===\n", display)
		cleanInNode := 0
		for _, r := range repos {
			grandTotal++
			exists, err := dirExists(r.Dest)
			if err != nil {
				return err
			}
			if !exists {
				fmt.Fprintf(out, "  MISSING  %s\n", r.Dest)
				continue
			}
			st := git.Check(r.Dest, r.Repo)
			if st.Clean {
				fmt.Fprintf(out, "  clean    %s\n", r.Dest)
				cleanInNode++
				grandClean++
				continue
			}
			fmt.Fprintf(out, "  DIRTY    %s\n", r.Dest)
			for _, reason := range st.Reasons {
				fmt.Fprintf(out, "             - %s\n", reason)
			}
		}
		fmt.Fprintf(out, "  (%d/%d clean in %s)\n", cleanInNode, len(repos), display)
	}
	if len(nodeOrder) > 1 {
		fmt.Fprintf(out, "\ntotal: %d/%d clean across %d node(s)\n", grandClean, grandTotal, len(nodeOrder))
	}
	return nil
}

// ── surgical removal ─────────────────────────────────────────────────────────

func surgicalRemove(repoDir string) (int, int, error) {
	tracked, err := git.ListTrackedFiles(repoDir)
	if err != nil {
		return 0, 0, err
	}

	removed := 0
	for _, rel := range tracked {
		full := filepath.Join(repoDir, rel)
		if err := os.Remove(full); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return removed, 0, err
		}
		removed++
	}

	if err := os.RemoveAll(filepath.Join(repoDir, ".git")); err != nil {
		return removed, 0, err
	}

	preserved, err := countRegularFiles(repoDir)
	if err != nil {
		return removed, 0, err
	}

	if err := pruneEmptyDirs(repoDir); err != nil {
		return removed, preserved, err
	}

	return removed, preserved, nil
}

func pruneEmptyDirs(root string) error {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			if err := pruneEmptyDirs(filepath.Join(root, e.Name())); err != nil {
				return err
			}
		}
	}
	entries, err = os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if len(entries) == 0 {
		return os.Remove(root)
	}
	return nil
}

func countRegularFiles(root string) (int, error) {
	count := 0
	err := filepath.Walk(root, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if !info.IsDir() {
			count++
		}
		return nil
	})
	return count, err
}

// ── helpers ──────────────────────────────────────────────────────────────────

type dirtyRepo struct {
	nodePath string
	status   git.CleanStatus
}

func printDirty(out io.Writer, dirty []dirtyRepo) {
	current := ""
	for _, d := range dirty {
		if d.nodePath != current {
			fmt.Fprintf(out, "  [%s]\n", d.nodePath)
			current = d.nodePath
		}
		fmt.Fprintf(out, "    %s\n", d.status.Repo)
		for _, r := range d.status.Reasons {
			fmt.Fprintf(out, "      - %s\n", r)
		}
	}
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
