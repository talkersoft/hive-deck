package ops

import (
	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/mcp"
)

func RunMcp(in McpInput) (string, error) {
	l, err := config.LoadDeck(in.Deck)
	if err != nil {
		return "", err
	}
	root, err := config.ExpandRoot(l.Setup.Workspace.Root)
	if err != nil {
		return "", err
	}
	return "", mcp.Apply(root, l)
}
