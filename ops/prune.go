package ops

import (
	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/prune"
)

func RunPrune(in PruneInput) (string, error) {
	l, err := config.LoadDeck(in.Deck)
	if err != nil {
		return "", err
	}
	return "", prune.Run(l, prune.Options{DryRun: in.DryRun})
}
