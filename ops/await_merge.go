package ops

import (
	"fmt"

	"github.com/talkersoft/hive-deck/internal/checkout"
	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/github"
	"github.com/talkersoft/hive-deck/internal/namegen"
	"github.com/talkersoft/hive-deck/internal/resolve"
)

type AwaitMergeInput struct {
	Op   string `json:"op"`
	Deck string `json:"deck"`
}

// RunAwaitMerge checks open PRs on the deck once and returns immediately.
// Returns "pending: N PR(s) open" if PRs remain, or "merged: → <branch>"
// when all are merged and the deck has transitioned.
// The caller (MCP /loop) drives the polling cadence — this function never sleeps.
func RunAwaitMerge(in AwaitMergeInput) (string, error) {
	l, err := config.LoadDeck(in.Deck)
	if err != nil {
		return "", err
	}

	plan, err := resolve.Build(l)
	if err != nil {
		return "", err
	}

	open := 0
	for _, repo := range plan.Repos {
		open += len(github.ListOpenPRs(repo.Dest))
	}

	if open > 0 {
		return fmt.Sprintf("pending: %d PR(s) open", open), nil
	}

	nextBranch := namegen.Generate()
	if err := checkout.Run(l, checkout.Options{
		RequireMergedPR: false,
		NextBranch:      nextBranch,
	}); err != nil {
		return "", err
	}
	return fmt.Sprintf("merged: all PRs merged → transitioned to %s", nextBranch), nil
}
