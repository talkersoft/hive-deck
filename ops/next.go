package ops

import (
	"github.com/talkersoft/hive-deck/internal/checkout"
	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/namegen"
)

func RunNext(in NextInput) (string, error) {
	l, err := config.LoadDeck(in.Deck)
	if err != nil {
		return "", err
	}
	nextBranch := in.Branch
	if nextBranch == "" {
		nextBranch = namegen.Generate()
	}
	// manual and await_merge both require PRs to be merged before transitioning.
	requireMerged := l.Setup.Ship.PRMode == config.PRModeManual || l.Setup.Ship.PRMode == config.PRModeAwaitMerge
	return "", checkout.Run(l, checkout.Options{
		RequireMergedPR: requireMerged,
		NextBranch:      nextBranch,
	})
}
