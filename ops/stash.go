package ops

import (
	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/stash"
)

func RunStashPush(in StashInput) (string, error) {
	l, err := config.LoadDeck(in.Deck)
	if err != nil {
		return "", err
	}
	return "", stash.Push(l)
}

func RunStashPop(in StashInput) (string, error) {
	l, err := config.LoadDeck(in.Deck)
	if err != nil {
		return "", err
	}
	return "", stash.Pop(l)
}
