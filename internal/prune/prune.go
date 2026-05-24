// Package prune implements `hv deck prune <workspace>`.
//
// Prune finds every git repo under the deck directory that is not declared in
// the deck YAML, verifies each is clean (committed and pushed), then removes
// them. Aborts before touching anything if any undeclared repo is dirty.
package prune

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/git"
	"github.com/talkersoft/hive-deck/internal/resolve"
	"github.com/talkersoft/hive-deck/internal/workspace"
)

type Options struct {
	Out    io.Writer
	DryRun bool
}

func Run(l *config.Loaded, opts Options) error {
	if opts.Out == nil {
		opts.Out = os.Stdout
	}

	if err := l.ValidateDeck(); err != nil {
		return err
	}

	plan, err := resolve.Build(l, "")
	if err != nil {
		return err
	}

	// Step 1: build set of declared repo destinations.
	allRepos, err := resolve.AllRepos(l)
	if err != nil {
		return err
	}
	declared := make(map[string]bool, len(allRepos))
	for _, r := range allRepos {
		declared[filepath.Clean(r.Dest)] = true
	}

	// Find all git repos on disk under the deck dir.
	var onDisk []string
	err = filepath.WalkDir(plan.DeckDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		if _, statErr := os.Stat(filepath.Join(path, ".git")); statErr == nil {
			onDisk = append(onDisk, path)
			return filepath.SkipDir // don't recurse into repos
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("scanning %s: %w", plan.DeckDir, err)
	}

	// Identify undeclared repos.
	var undeclared []string
	for _, dir := range onDisk {
		if !declared[filepath.Clean(dir)] {
			undeclared = append(undeclared, dir)
		}
	}

	if len(undeclared) == 0 {
		fmt.Fprintln(opts.Out, "prune: nothing to remove — all on-disk repos are declared")
		return nil
	}

	fmt.Fprintf(opts.Out, "prune: found %d undeclared repo(s):\n", len(undeclared))
	for _, dir := range undeclared {
		fmt.Fprintf(opts.Out, "  %s\n", rel(plan.DeckDir, dir))
	}

	// Step 2: verify all undeclared repos are clean before touching anything.
	var dirty []git.CleanStatus
	for _, dir := range undeclared {
		st := git.Check(dir, filepath.Base(dir))
		if !st.Clean {
			dirty = append(dirty, st)
		}
	}
	if len(dirty) > 0 {
		fmt.Fprintf(opts.Out, "\nprune aborted — %d repo(s) have unpushed or uncommitted work:\n", len(dirty))
		for _, st := range dirty {
			fmt.Fprintf(opts.Out, "  %s\n", st.Repo)
			for _, reason := range st.Reasons {
				fmt.Fprintf(opts.Out, "    - %s\n", reason)
			}
		}
		return fmt.Errorf("clean or push the repos above, then retry")
	}

	if opts.DryRun {
		fmt.Fprintln(opts.Out, "\n(dry-run: no files removed)")
		return nil
	}

	// Step 3: remove them.
	for _, dir := range undeclared {
		if err := os.RemoveAll(dir); err != nil {
			return fmt.Errorf("removing %s: %w", rel(plan.DeckDir, dir), err)
		}
		fmt.Fprintf(opts.Out, "  removed %s\n", rel(plan.DeckDir, dir))
	}

	if err := workspace.Regenerate(plan, l); err != nil {
		return err
	}
	fmt.Fprintln(opts.Out, "regenerated .code-workspace file")
	return nil
}

func rel(base, path string) string {
	r := strings.TrimPrefix(path, base+string(filepath.Separator))
	if r == path {
		return path
	}
	return r
}
