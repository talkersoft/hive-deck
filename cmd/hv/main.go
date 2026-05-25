// Command hv is the hive-deck CLI — provision and teardown developer decks.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/talkersoft/hive-deck/internal/checkout"
	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/pr"
	"github.com/talkersoft/hive-deck/internal/provision"
	"github.com/talkersoft/hive-deck/internal/prune"
	"github.com/talkersoft/hive-deck/internal/sync"
	"github.com/talkersoft/hive-deck/internal/teardown"
)

func main() {
	root := &cobra.Command{
		Use:           "hv",
		Short:         "hive — developer workspace and workflow tooling",
		SilenceUsage:  true,
		SilenceErrors: true,
		Long: `hv manages on-disk developer decks from a YAML deck file,
parameterized by per-machine config in config.yaml.

Config files (config.yaml, modules.yaml, deck files) are resolved in order:
  1. $HV_HOME/.hv/  — explicit override (useful in CI or dev)
  2. <CWD>/.hv/     — project-local config, wins when present
  3. $HOME/.hv/     — global user install fallback

The deck name is the positional argument; it maps to <name>.yaml found via
the same search order.`,
	}

	root.AddCommand(
		provisionCmd(),
		syncCmd(),
		teardownCmd(),
		pruneCmd(),
		statusCmd(),
		listCmd(),
		decksCmd(),
		prCmd(),
		defaultCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func provisionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "provision <deck>",
		Short: "Provision every repo in the deck (idempotent)",
		Long: `Provision every declared repo in the deck.

Idempotent — safe to run any number of times:
  missing repo     → cloned
  dir without .git → restored in place (untracked files preserved)
  already cloned   → skipped

GitHub create-if-missing is always on: for each repo that doesn't yet exist
on github.com, ` + "`gh repo create --private --add-readme`" + ` runs before the clone.
The ` + "`gh`" + ` CLI is a hard runtime dependency.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			l, err := config.LoadDeck(args[0])
			if err != nil {
				return err
			}
			return provision.Run(l, provision.Options{})
		},
	}
}

func teardownCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "teardown <deck>",
		Short: "Surgically remove tracked files + .git/ from every repo; preserve untracked files",
		Long: `Surgical preserve teardown — the only teardown hv offers.

For each repo: deletes git-tracked files and the .git/ folder,
leaves every untracked file in place, prunes now-empty directories.

Refuses if any repo has tracked work that would be lost.
There is no --force or nuke mode. Use ` + "`rm -rf`" + ` for destructive wipes.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			l, err := config.LoadDeck(args[0])
			if err != nil {
				return err
			}
			return teardown.Run(l, teardown.Options{})
		},
	}
}

func syncCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "sync <deck>",
		Short: "Pull every repo after verifying all are clean",
		Long: `Sync verifies every repo is clean (committed and pushed), then
runs git pull on each one. Aborts before touching anything if any repo
has uncommitted changes, unpushed commits, or stash entries.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			l, err := config.LoadDeck(args[0])
			if err != nil {
				return err
			}
			return sync.Run(l, sync.Options{})
		},
	}
}

func pruneCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "prune <deck> [--dry-run]",
		Short: "Remove on-disk repos not declared in the deck",
		Long: `Prune finds every git repo under the deck directory that is not declared
in the deck YAML. It runs in three steps:

  1. Identify all undeclared repos on disk
  2. Verify every undeclared repo is clean (committed and pushed)
  3. Remove them — aborts if any are dirty

Use --dry-run to preview what would be removed without removing anything.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			l, err := config.LoadDeck(args[0])
			if err != nil {
				return err
			}
			return prune.Run(l, prune.Options{DryRun: dryRun})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show what would be removed without removing anything")
	return cmd
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <deck>",
		Short: "Report git state of every repo in the deck",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			l, err := config.LoadDeck(args[0])
			if err != nil {
				return err
			}
			return teardown.Status(l, os.Stdout)
		},
	}
}

func listCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list <deck>",
		Short: "List every repo declared by the deck with provisioned state",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			l, err := config.LoadDeck(args[0])
			if err != nil {
				return err
			}
			root, err := config.ExpandRoot(l.Setup.DecksRoot)
			if err != nil {
				return err
			}
			wsDir := filepath.Join(root, l.DeckName)
			if err := l.ValidateDeck(); err != nil {
				return err
			}
			fmt.Printf("%-40s %-25s %s\n", "DEST", "MODULE", "PROVISIONED")
			return walkListNode(l.DeckFile.Deck, wsDir, "", l)
		},
	}
}

func walkListNode(node config.TreeNode, nodeDir, nodePath string, l *config.Loaded) error {
	wsPrefix := filepath.Join(func() string {
		r, _ := config.ExpandRoot(l.Setup.DecksRoot)
		return r
	}(), l.DeckName) + "/"

	for _, ref := range node.RepoRefs {
		parts := strings.SplitN(ref, "/", 2)
		repoName := parts[1]
		dest := filepath.Join(nodeDir, repoName)
		prov := "no"
		if fi, err := os.Stat(dest); err == nil && fi.IsDir() {
			prov = "yes"
		}
		rel := strings.TrimPrefix(dest, wsPrefix)
		fmt.Printf("%-40s %-25s %s\n", rel, ref, prov)
	}

	for _, modName := range node.ModuleRefs {
		mod := l.Modules[modName]
		for _, repo := range mod.Repos {
			dest := filepath.Join(nodeDir, repo)
			prov := "no"
			if fi, err := os.Stat(dest); err == nil && fi.IsDir() {
				prov = "yes"
			}
			rel := strings.TrimPrefix(dest, wsPrefix)
			fmt.Printf("%-40s %-25s %s\n", rel, modName, prov)
		}
	}

	childNames := sortedStringKeys(node.Children)
	for _, childName := range childNames {
		child := node.Children[childName]
		childPath := childName
		if nodePath != "" {
			childPath = nodePath + "/" + childName
		}
		if err := walkListNode(child, filepath.Join(nodeDir, childName), childPath, l); err != nil {
			return err
		}
	}
	return nil
}

func decksCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "decks",
		Short: "List every deck file (*.yaml) in ~/.hv/",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			home, _, err := config.LoadSetup()
			if err != nil {
				return err
			}
			matches, err := filepath.Glob(filepath.Join(home, config.ConfigDir, "*.yaml"))
			if err != nil {
				return err
			}
			sort.Strings(matches)
			for _, m := range matches {
				base := filepath.Base(m)
				if base == config.SetupFile || strings.HasSuffix(base, ".example") {
					continue
				}
				fmt.Println(strings.TrimSuffix(base, ".yaml"))
			}
			return nil
		},
	}
}

func prCmd() *cobra.Command {
	var title, body string
	cmd := &cobra.Command{
		Use:   "pr <deck> --title <title> [--body <body>]",
		Short: "Open a pull request for every repo whose branch is ahead of origin/<default>",
		Long: `For every provisioned repo in the deck:

  dirty working tree          → abort (all repos checked first)
  on default branch + ahead   → abort (create a branch for this work)
  on default branch, no ahead → skip
  on feature branch + ahead   → create PR with the given title/body
  on feature branch, no ahead → skip
  PR already open             → skip

The same title and body are applied to every PR created.
All created PR URLs are printed at the end.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if title == "" {
				return fmt.Errorf("--title is required")
			}
			l, err := config.LoadDeck(args[0])
			if err != nil {
				return err
			}
			return pr.Run(l, pr.Options{
				Title: title,
				Body:  body,
			})
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "PR title (required)")
	cmd.Flags().StringVar(&body, "body", "", "PR body/description")
	return cmd
}

func defaultCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "default <deck>",
		Short: "Switch every repo to its default branch after verifying all are clean",
		Long: `Verifies every repo in the deck is fully clean (committed, pushed, no stash,
no detached HEAD), then switches each one to its default branch and pulls.

Aborts before touching anything if any repo fails the clean check.
Repos already on the default branch are pulled but not checked out.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			l, err := config.LoadDeck(args[0])
			if err != nil {
				return err
			}
			return checkout.Run(l, checkout.Options{})
		},
	}
}

func sortedStringKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
