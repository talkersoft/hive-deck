package ops

import (
	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/sync"
)

func RunSync(in SyncInput) (string, error) {
	l, err := config.LoadDeck(in.Deck)
	if err != nil {
		return "", err
	}
	return "", sync.Run(l, sync.Options{})
}
