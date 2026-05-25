// Package sync implements `hv sync <deck>`.
//
// Sync checks every in-scope repo is clean (committed and pushed), then pulls
// each one. Aborts before touching anything if any repo is dirty.
package sync

import (
	"fmt"
	"io"
	"os"

	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/git"
	"github.com/talkersoft/hive-deck/internal/resolve"
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

	// Only operate on repos that exist on disk.
	var present []resolve.RepoPlan
	for _, r := range plan.Repos {
		if fi, err := os.Stat(r.Dest); err == nil && fi.IsDir() {
			present = append(present, r)
		} else {
			fmt.Fprintf(opts.Out, "  skip [%s/%s]: not on disk\n", r.NodePath, r.Repo)
		}
	}

	if len(present) == 0 {
		fmt.Fprintln(opts.Out, "sync: nothing to sync — no repos on disk")
		return nil
	}

	// Check all repos are clean before pulling any.
	var dirty []dirtyRepo
	for _, r := range present {
		st := git.Check(r.Dest, r.Repo)
		if !st.Clean {
			dirty = append(dirty, dirtyRepo{nodePath: r.NodePath, status: st})
		}
	}
	if len(dirty) > 0 {
		fmt.Fprintf(opts.Out, "sync aborted — %d repo(s) are not clean:\n", len(dirty))
		current := ""
		for _, d := range dirty {
			if d.nodePath != current {
				fmt.Fprintf(opts.Out, "  [%s]\n", d.nodePath)
				current = d.nodePath
			}
			fmt.Fprintf(opts.Out, "    %s\n", d.status.Repo)
			for _, reason := range d.status.Reasons {
				fmt.Fprintf(opts.Out, "      - %s\n", reason)
			}
		}
		return fmt.Errorf("commit, push, or stash the repos above, then retry")
	}

	fmt.Fprintf(opts.Out, "sync: %s repos=%d\n", plan.Deck, len(present))
	for _, r := range present {
		fmt.Fprintf(opts.Out, "  [%s] %s\n", r.NodePath, r.Repo)
		if err := git.Pull(r.Dest); err != nil {
			return err
		}
	}
	return nil
}

type dirtyRepo struct {
	nodePath string
	status   git.CleanStatus
}
