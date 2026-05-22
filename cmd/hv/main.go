// Command hv is the hive-deck CLI — provision and teardown developer decks.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/provision"
	"github.com/talkersoft/hive-deck/internal/teardown"
)

func main() {
	root := &cobra.Command{
		Use:          "hv",
		Short:        "hive — developer workspace and workflow tooling",
		SilenceUsage: true,
	}

	root.AddCommand(deckCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func deckCmd() *cobra.Command {
	deck := &cobra.Command{
		Use:   "deck",
		Short: "Provision and teardown multi-repo developer decks",
		Long: `hv deck manages on-disk developer decks from a YAML deck file,
parameterized by per-machine config in setup.yaml.

Config files (setup.yaml, modules.yaml, deck files) are resolved in order:
  1. $HV_HOME/.hive/  — explicit override (useful in CI or dev)
  2. <CWD>/.hive/     — project-local config, wins when present
  3. $HOME/.hive/     — global user install fallback

The deck name is the positional argument; it maps to <name>.yaml found via
the same search order.

Provision runs are transactional — if any clone fails, every clone made
during the run is rolled back. Teardown is surgical preserve mode: tracked
files + .git/ go, untracked files stay.`,
		SilenceUsage: true,
	}

	deck.AddCommand(
		provisionCmd(),
		teardownCmd(),
		statusCmd(),
		listCmd(),
		decksCmd(),
	)

	return deck
}

func provisionCmd() *cobra.Command {
	var filter string
	var cloneMissing bool
	cmd := &cobra.Command{
		Use:   "provision <workspace> [--filter <node>]",
		Short: "Clone every repo in the workspace as one transaction",
		Long: `Clone every declared repo for the in-scope workspace (or a filtered subtree).

GitHub create-if-missing is always on: for each repo that doesn't yet exist
on github.com, ` + "`gh repo create --private --add-readme`" + ` runs before the clone.
The ` + "`gh`" + ` CLI is a hard runtime dependency.

Default (strict): fails if any declared repo dir already exists on disk.
--clone-missing: additive — clones only the absent repos.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			l, err := config.LoadDeck(args[0])
			if err != nil {
				return err
			}
			return provision.Run(l, provision.Options{
				Filter:       filter,
				CloneMissing: cloneMissing,
			})
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "", "provision only the subtree rooted at a node with this name")
	cmd.Flags().BoolVar(&cloneMissing, "clone-missing", false, "additive mode: clone only repos that don't yet exist on disk")
	return cmd
}

func teardownCmd() *cobra.Command {
	var filter string
	cmd := &cobra.Command{
		Use:   "teardown <workspace> [--filter <node>]",
		Short: "Surgically remove tracked files + .git/ from every in-scope repo; preserve untracked files",
		Long: `Surgical preserve teardown — the only teardown hv deck offers.

For each in-scope repo: deletes git-tracked files and the .git/ folder,
leaves every untracked file in place, prunes now-empty directories.

Refuses if any in-scope repo has tracked work that would be lost.
There is no --force or nuke mode. Use ` + "`rm -rf`" + ` for destructive wipes.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			l, err := config.LoadDeck(args[0])
			if err != nil {
				return err
			}
			return teardown.Run(l, teardown.Options{
				Filter: filter,
			})
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "", "teardown only the subtree rooted at a node with this name")
	return cmd
}

func statusCmd() *cobra.Command {
	var filter string
	cmd := &cobra.Command{
		Use:   "status <workspace> [--filter <node>]",
		Short: "Report git state of every repo in the workspace",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			l, err := config.LoadDeck(args[0])
			if err != nil {
				return err
			}
			return teardown.Status(l, filter, os.Stdout)
		},
	}
	cmd.Flags().StringVar(&filter, "filter", "", "show only the subtree rooted at a node with this name")
	return cmd
}

func listCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list <workspace>",
		Short: "List every repo declared by the workspace with provisioned state",
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
	return cmd
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
		Short: "List every deck file (*.yaml) in ~/.hive/",
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

func sortedStringKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
