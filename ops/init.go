package ops

import (
	"fmt"
	"os"

	"github.com/talkersoft/hive-deck/internal/branch"
	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/git"
	"github.com/talkersoft/hive-deck/internal/mcp"
	"github.com/talkersoft/hive-deck/internal/namegen"
	"github.com/talkersoft/hive-deck/internal/provision"
	"github.com/talkersoft/hive-deck/internal/resolve"
)

func RunInit(in InitInput) (string, error) {
	branchName := in.Branch
	if branchName == "" {
		branchName = namegen.Generate()
	}
	l, err := config.LoadDeck(in.Deck)
	if err != nil {
		return "", err
	}
	if err := branch.PreFlightExisting(l, branchName); err != nil {
		return "", err
	}
	if err := provision.Run(l, provision.Options{}); err != nil {
		return "", err
	}

	// Ensure the deck's configured branch exists on every remote before branching.
	if l.DeckFile.Branch != "" {
		plan, err := resolve.Build(l)
		if err != nil {
			return "", err
		}
		for _, r := range plan.Repos {
			if ensureErr := git.EnsureRemoteBranch(r.Dest, l.DeckFile.Branch); ensureErr != nil {
				fmt.Fprintf(os.Stderr, "  warn    %-30s ensure remote branch %s: %v\n", r.Repo, l.DeckFile.Branch, ensureErr)
			}
		}
	}

	if err := branch.CreateAll(l, branchName); err != nil {
		return "", err
	}
	root, err := config.ExpandRoot(l.Setup.Workspace.Root)
	if err != nil {
		return "", err
	}
	if err := mcp.Apply(root, l); err != nil {
		return "", err
	}
	return "", nil
}
