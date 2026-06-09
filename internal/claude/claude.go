// Package claude provisions per-repo .claude/settings.local.json files when
// the config.yaml enables it.
package claude

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/talkersoft/hive-deck/internal/config"
)

const (
	settingsDir  = ".claude"
	settingsFile = "settings.local.json"
)

// MaybeWrite writes settings.local.json into repoDir/.claude/ when cs.Enabled is true.
// Mode controls behaviour: "overwrite" (default) always replaces; "skip" skips if the
// file already exists; "merge" unions allow lists and preserves existing defaultMode.
// No-op when cs.Enabled is false.
func MaybeWrite(repoDir string, cs config.ClaudeSettings) error {
	if !cs.Enabled {
		return nil
	}
	if len(cs.Permissions.Allow) == 0 {
		return errors.New("claude_settings.enabled=true but claude_settings.permissions.allow is empty")
	}
	return writeSettingsFile(repoDir, cs)
}

func writeSettingsFile(repoDir string, cs config.ClaudeSettings) error {
	dir := filepath.Join(repoDir, settingsDir)
	path := filepath.Join(dir, settingsFile)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	switch cs.Mode {
	case "skip":
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		doc := map[string]any{"permissions": cs.Permissions}
		b, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return err
		}
		b = append(b, '\n')
		return os.WriteFile(path, b, 0o644)

	case "merge":
		perms := cs.Permissions
		if data, err := os.ReadFile(path); err == nil {
			var existing struct {
				Permissions config.ClaudePermissions `json:"permissions"`
			}
			if json.Unmarshal(data, &existing) == nil {
				seen := map[string]bool{}
				for _, e := range existing.Permissions.Allow {
					seen[e] = true
				}
				merged := existing.Permissions.Allow
				for _, e := range cs.Permissions.Allow {
					if !seen[e] {
						merged = append(merged, e)
					}
				}
				perms = existing.Permissions
				perms.Allow = merged
				if cs.Permissions.DefaultMode != "" {
					perms.DefaultMode = cs.Permissions.DefaultMode
				}
			}
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("read %s: %w", path, err)
		}
		doc := map[string]any{"permissions": perms}
		b, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return err
		}
		b = append(b, '\n')
		return os.WriteFile(path, b, 0o644)

	default: // "overwrite" or ""
		doc := map[string]any{"permissions": cs.Permissions}
		b, err := json.MarshalIndent(doc, "", "  ")
		if err != nil {
			return err
		}
		b = append(b, '\n')
		return os.WriteFile(path, b, 0o644)
	}
}
