// Command hv is the hive-deck CLI — provision and teardown developer decks.
package main

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/talkersoft/hive-deck/ops"
)

func main() {
	// If stdin is piped, treat the payload as a JSON dispatch call.
	stat, _ := os.Stdin.Stat()
	if (stat.Mode() & os.ModeCharDevice) == 0 {
		payload, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error: reading stdin:", err)
			os.Exit(1)
		}
		out, err := ops.Dispatch(payload)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		fmt.Print(out)
		return
	}

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
		promoteCmd(),
		planCmd(),
	)

	if err := root.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func initCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init <deck> [branch]",
		Short: "Provision every repo in the deck and create a feature branch",
		Long: `Provision every declared repo and create a feature branch across all of them.

Provision is idempotent:
  missing repo     → cloned
  dir without .git → restored in place (untracked files preserved)
  already cloned   → skipped

For already-provisioned repos, requires:
  - on default branch (run hv next first if not)
  - working tree clean and fully pushed
  - branch name does not already exist in any repo

After provision, every repo is checked out on <branch>.
If <branch> is omitted, a name is generated automatically.
GitHub create-if-missing is always on.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			in := ops.InitInput{Op: "init", Deck: args[0]}
			if len(args) == 2 {
				in.Branch = args[1]
			}
			_, err := ops.RunInit(in)
			return err
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
			out, err := ops.RunShip(ops.ShipInput{
				Op:      "ship",
				Deck:    args[0],
				Message: args[1],
				Title:   title,
				Body:    body,
			})
			if out != "" {
				fmt.Print(out)
			}
			return err
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
			_, err := ops.RunTeardown(ops.TeardownInput{Op: "teardown", Deck: args[0]})
			return err
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
			_, err := ops.RunSync(ops.SyncInput{Op: "sync", Deck: args[0]})
			return err
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
			_, err := ops.RunPrune(ops.PruneInput{Op: "prune", Deck: args[0], DryRun: dryRun})
			return err
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
			out, err := ops.RunStatus(ops.StatusInput{Op: "status", Deck: args[0]})
			if out != "" {
				fmt.Print(out)
			}
			return err
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
			out, err := ops.RunListRepos(ops.ListReposInput{Op: "list_repos", Deck: args[0]})
			if out != "" {
				fmt.Print(out)
			}
			return err
		},
	}
}

func listPullsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pulls <deck>",
		Short: "Show all open pull requests across every provisioned repo in the deck",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			out, err := ops.RunListPulls(ops.ListPullsInput{Op: "list_pulls", Deck: args[0]})
			if out != "" {
				fmt.Print(out)
			}
			return err
		},
	}
}

func listDecksCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "decks",
		Short: "List every deck file (*.yaml) in ~/.hv/decks/",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			out, err := ops.RunDecks(ops.DecksInput{Op: "decks"})
			if out != "" {
				fmt.Print(out)
			}
			return err
		},
	}
}

func nextCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "next <deck> [branch]",
		Short: "Transition every repo to a new branch based on origin/<default>",
		Long: `Transition from the current feature branch to a new one based on origin/<default>.

All repos must be clean, pushed, and (when require_merged_pr is on) PRs merged.
Fetches origin and creates <branch> from origin/<default> — local main is never checked out.
Refuses if <branch> equals the default branch name (main/master).
If <branch> is omitted, a name is generated automatically.`,
		Args: cobra.RangeArgs(1, 2),
		RunE: func(_ *cobra.Command, args []string) error {
			in := ops.NextInput{Op: "next", Deck: args[0]}
			if len(args) == 2 {
				in.Branch = args[1]
			}
			_, err := ops.RunNext(in)
			return err
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
			_, err := ops.RunStashPush(ops.StashInput{Op: "stash_push", Deck: args[0]})
			return err
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
			_, err := ops.RunStashPop(ops.StashInput{Op: "stash_pop", Deck: args[0]})
			return err
		},
	}
}

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp <deck>",
		Short: "Write MCP server config to the deck's .mcp.json",
		Long: `Resolves the MCP registries listed in the deck file against ~/.hv/mcps.yaml and
writes an mcpServers block into {deck.root}/{deck}/.mcp.json.
When deck.enableRootMCP is true (default), also merges into {deck.root}/.mcp.json.
All other keys in those files are preserved.

Requires mcp_manager.enabled: true in config.yaml.
Also runs automatically at the end of hv init when enabled.`,
		Args: cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			_, err := ops.RunMcp(ops.McpInput{Op: "mcp", Deck: args[0]})
			return err
		},
	}
}

func promoteCmd() *cobra.Command {
	var listFlag bool
	cmd := &cobra.Command{
		Use:   "promote <deck> <workflowName>",
		Short: "Promote a reviewed plan into an executable orchestration workflow",
		Long: `Promotes a reviewed plan into an executable orchestration workflow.

  hv promote <deck> <workflowName>   Run named workflow extension for the deck
  hv promote --list <deck>           List available workflows for the deck`,
		Args: func(cmd *cobra.Command, args []string) error {
			if listFlag {
				return cobra.ExactArgs(1)(cmd, args)
			}
			return cobra.ExactArgs(2)(cmd, args)
		},
		RunE: func(_ *cobra.Command, args []string) error {
			deck := args[0]
			workflowName := ""
			if len(args) == 2 {
				workflowName = args[1]
			}
			out, err := ops.RunWorkflow(ops.WorkflowInput{
				Op: "orchestrate", Type: "workflow", Deck: deck, WorkflowName: workflowName, List: listFlag,
			})
			if out != "" {
				fmt.Print(out)
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&listFlag, "list", false, "List available workflows for the deck")
	return cmd
}

func planCmd() *cobra.Command {
	var listFlag bool
	cmd := &cobra.Command{
		Use:   "plan <deck> <workflowName>",
		Short: "Assemble and print plan instructions",
		Long: `Assembles plan instructions for the given deck and prints them to stdout.

  hv plan <deck> <workflowName>   Run named plan workflow for the deck
  hv plan --list <deck>           List available plan workflows for the deck`,
		Args: func(cmd *cobra.Command, args []string) error {
			if listFlag {
				return cobra.ExactArgs(1)(cmd, args)
			}
			return cobra.ExactArgs(2)(cmd, args)
		},
		RunE: func(_ *cobra.Command, args []string) error {
			deck := args[0]
			workflowName := ""
			if len(args) == 2 {
				workflowName = args[1]
			}
			out, err := ops.RunWorkflow(ops.WorkflowInput{
				Op: "orchestrate", Type: "plan", Deck: deck, WorkflowName: workflowName, List: listFlag,
			})
			if out != "" {
				fmt.Print(out)
			}
			return err
		},
	}
	cmd.Flags().BoolVar(&listFlag, "list", false, "List available plan workflows for the deck")
	return cmd
}


