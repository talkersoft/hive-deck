// Command hv is the hive-deck CLI — provision and teardown developer decks.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/talkersoft/hive-deck/internal/branch"
	"github.com/talkersoft/hive-deck/internal/checkout"
	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/github"
	"github.com/talkersoft/hive-deck/internal/mcp"
	"github.com/talkersoft/hive-deck/internal/provision"
	"github.com/talkersoft/hive-deck/internal/prune"
	"github.com/talkersoft/hive-deck/internal/resolve"
	"github.com/talkersoft/hive-deck/internal/ship"
	"github.com/talkersoft/hive-deck/internal/stash"
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
		initCmd(),
		shipCmd(),
		syncCmd(),
		teardownCmd(),
		pruneCmd(),
		statusCmd(),
		listGroupCmd(),
		nextCmd(),
		stashGroupCmd(),
		mcpCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init <deck> <branch>",
		Short: "Provision every repo in the deck and create a feature branch",
		Long: `Provision every declared repo and create a feature branch across all of them.

Provision is idempotent:
  missing repo     → cloned
  dir without .git → restored in place (untracked files preserved)
  already cloned   → skipped

For already-provisioned repos, requires:
  - on default branch (run hv default first if not)
  - working tree clean and fully pushed
  - branch name does not already exist in any repo

After provision, every repo is checked out on <branch>.
GitHub create-if-missing is always on.`,
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			deck, branchName := args[0], args[1]
			l, err := config.LoadDeck(deck)
			if err != nil {
				return err
			}
			if err := branch.PreFlightExisting(l, branchName); err != nil {
				return err
			}
			if err := provision.Run(l, provision.Options{}); err != nil {
				return err
			}
			if err := branch.CreateAll(l, branchName); err != nil {
				return err
			}
			root, err := config.ExpandRoot(l.Setup.Deck.Root)
			if err != nil {
				return err
			}
			return mcp.Apply(root, l)
		},
	}
}

func shipCmd() *cobra.Command {
	var title, body string
	cmd := &cobra.Command{
		Use:   "ship <deck> <message> --title <title> [--body <body>]",
		Short: "Commit, push, and open pull requests for every repo in the deck",
		Long: `For every repo in the deck:
  has uncommitted changes  → git add -A + git commit -m <message> + git push
  committed but not pushed → git push
  ahead of origin/<default> → open PR with --title and --body
  already clean and pushed  → skip

Refuses to run if any repo is on the default branch — feature work only.
Automatically sets upstream on first push so the branch is tracked on remote.`,
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			if title == "" {
				return fmt.Errorf("--title is required")
			}
			l, err := config.LoadDeck(args[0])
			if err != nil {
				return err
			}
			if err := ship.Run(l, ship.Options{
				Message:             args[1],
				Title:               title,
				Body:                body,
				DeleteBranchOnMerge: l.Setup.Ship.DeleteBranchOnMerge,
				AutoMerge:           l.Setup.Ship.AutoMerge,
			}); err != nil {
				return err
			}
			if l.Setup.Ship.TeardownOnShip && l.Setup.Ship.AutoMerge {
				fmt.Println()
				return teardown.Run(l, teardown.Options{RequireMergedPR: false})
			}
			if l.Setup.Ship.RequireMergedPR && !l.Setup.Ship.AutoMerge {
				fmt.Println("\nmerge-gate: PRs opened — merge them, then call hv next <deck> <branch> to transition")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "PR title (required)")
	cmd.Flags().StringVar(&body, "body", "", "PR body/description")
	return cmd
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
			return teardown.Run(l, teardown.Options{RequireMergedPR: l.Setup.Ship.RequireMergedPR})
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

func listGroupCmd() *cobra.Command {
	grp := &cobra.Command{
		Use:   "list",
		Short: "List repos, open pulls, or available decks",
	}
	grp.AddCommand(listReposCmd(), listPullsCmd(), listDecksCmd())
	return grp
}

func listReposCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "repos <deck>",
		Short: "List every repo declared by the deck with provisioned state",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			l, err := config.LoadDeck(args[0])
			if err != nil {
				return err
			}
			root, err := config.ExpandRoot(l.Setup.Deck.Root)
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

func listPullsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pulls <deck>",
		Short: "Show all open pull requests across every provisioned repo in the deck",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			l, err := config.LoadDeck(args[0])
			if err != nil {
				return err
			}
			if err := l.ValidateDeck(); err != nil {
				return err
			}
			plan, err := resolve.Build(l)
			if err != nil {
				return err
			}
			total := 0
			for _, repo := range plan.Repos {
				if fi, serr := os.Stat(repo.Dest); serr != nil || !fi.IsDir() {
					continue
				}
				prs := github.ListOpenPRs(repo.Dest)
				for _, pr := range prs {
					fmt.Printf("#%-4d %-30s %s\n", pr.Number, repo.Repo, pr.URL)
					fmt.Printf("      branch: %-28s %s\n", pr.Branch, pr.Title)
					total++
				}
			}
			if total == 0 {
				fmt.Println("no open pull requests")
			}
			return nil
		},
	}
}

func listDecksCmd() *cobra.Command {
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
				if base == config.SetupFile || base == config.ModulesFile || base == config.MCPsFile || strings.HasSuffix(base, ".example") {
					continue
				}
				fmt.Println(strings.TrimSuffix(base, ".yaml"))
			}
			return nil
		},
	}
}

func walkListNode(node config.TreeNode, nodeDir, nodePath string, l *config.Loaded) error {
	wsPrefix := filepath.Join(func() string {
		r, _ := config.ExpandRoot(l.Setup.Deck.Root)
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

func nextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "next <deck> <branch>",
		Short: "Transition every repo to a new branch based on origin/<default>",
		Long: `Transition from the current feature branch to a new one based on origin/<default>.

All repos must be clean, pushed, and (when require_merged_pr is on) PRs merged.
Fetches origin and creates <branch> from origin/<default> — local main is never checked out.
Refuses if <branch> equals the default branch name (main/master).`,
		Args: cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			l, err := config.LoadDeck(args[0])
			if err != nil {
				return err
			}
			return checkout.Run(l, checkout.Options{
				RequireMergedPR: l.Setup.Ship.RequireMergedPR,
				NextBranch:      args[1],
			})
		},
	}
}

func stashGroupCmd() *cobra.Command {
	grp := &cobra.Command{
		Use:   "stash",
		Short: "Stash/restore uncommitted changes across repos (deadlock escape hatch)",
	}
	grp.AddCommand(stashPushCmd(), stashPopCmd())
	return grp
}

func stashPushCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "push <deck>",
		Short: "Stash uncommitted changes across all repos with a merged PR",
		Long: `Escape hatch for the deadlock where a branch has a merged PR but uncommitted
changes block hv next. Runs git stash push on every dirty repo.
Every dirty repo must have a merged or closed PR — otherwise use hv ship.
After stashing: run hv next to transition, then hv stash pop to restore.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			l, err := config.LoadDeck(args[0])
			if err != nil {
				return err
			}
			return stash.Push(l)
		},
	}
}

func stashPopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pop <deck>",
		Short: "Restore stashed changes across all repos that have a stash entry",
		Long:  `Runs git stash pop on every repo that has a stash entry. Use after hv next.`,
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			l, err := config.LoadDeck(args[0])
			if err != nil {
				return err
			}
			return stash.Pop(l)
		},
	}
}

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp <deck>",
		Short: "Write MCP server config to the deck's .claude/settings.json",
		Long: `Resolves the MCP registries listed in the deck file against ~/.hv/mcps.yaml and
writes an mcpServers block into {deck.root}/{deck}/.claude/settings.json.
When deck.enableRootMCP is true (default), also merges into {deck.root}/.claude/settings.json.
All other keys in those files are preserved.

Requires mcp_manager.enabled: true in config.yaml.
Also runs automatically at the end of hv init when enabled.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			l, err := config.LoadDeck(args[0])
			if err != nil {
				return err
			}
			root, err := config.ExpandRoot(l.Setup.Deck.Root)
			if err != nil {
				return err
			}
			return mcp.Apply(root, l)
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
