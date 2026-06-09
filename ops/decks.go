package ops

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/talkersoft/hive-deck/internal/config"
)

func RunDecks(in DecksInput) (string, error) {
	if _, _, err := config.LoadSetup(); err != nil {
		return "", err
	}
	decksDir, err := config.FindDecksDir()
	if err != nil {
		return "", err
	}
	if decksDir == "" {
		return "", nil
	}
	matches, err := filepath.Glob(filepath.Join(decksDir, "*.yaml"))
	if err != nil {
		return "", err
	}
	sort.Strings(matches)
	var sb strings.Builder
	for _, m := range matches {
		base := filepath.Base(m)
		sb.WriteString(strings.TrimSuffix(base, ".yaml"))
		sb.WriteByte('\n')
	}
	return sb.String(), nil
}
