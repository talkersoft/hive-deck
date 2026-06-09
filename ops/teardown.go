package ops

import (
	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/teardown"
)

func RunTeardown(in TeardownInput) (string, error) {
	l, err := config.LoadDeck(in.Deck)
	if err != nil {
		return "", err
	}
	requireMerged := l.Setup.Ship.PRMode == config.PRModeManual
	return "", teardown.Run(l, teardown.Options{RequireMergedPR: requireMerged})
}
