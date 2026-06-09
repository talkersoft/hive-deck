package ops

import (
	"bytes"

	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/teardown"
)

func RunStatus(in StatusInput) (string, error) {
	l, err := config.LoadDeck(in.Deck)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := teardown.Status(l, &buf); err != nil {
		return "", err
	}
	return buf.String(), nil
}
