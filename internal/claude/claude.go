// Package claude provisions per-repo .claude/settings.local.json files when
// the config.yaml enables it. The write is idempotent: if the file already
// exists its content is merged with the configured settings rather than
// overwritten — unknown keys and user-added entries are always preserved.
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
	// Matches the bypassPermissions string Claude Code's settings schema uses
	// when set; we expose it as the default so users don't have to spell it.
	defaultMode = "bypassPermissions"
)

// MaybeWrite merges settings.local.json into repoDir/.claude/ when cs.Enabled
// is true. If the file already exists, only entries missing from the allow list
// are appended and defaultMode is set only when absent — nothing is removed.
// No-op entirely when cs.Enabled is false.
func MaybeWrite(repoDir string, cs config.ClaudeSettings) error {
	if !cs.Enabled {
		return nil
	}
	if len(cs.Allow) == 0 {
		return errors.New("claude_settings.enabled=true but claude_settings.allow is empty")
	}
	return writeSettingsFile(repoDir, cs)
}

func writeSettingsFile(repoDir string, cs config.ClaudeSettings) error {
	dir := filepath.Join(repoDir, settingsDir)
	path := filepath.Join(dir, settingsFile)

	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	// Read and parse existing file, or start fresh.
	doc := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}

	mergePermissions(doc, cs)

	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}

// mergePermissions adds any allow entries not already present and sets
// defaultMode only when the key is absent. All other keys in doc are untouched.
func mergePermissions(doc map[string]any, cs config.ClaudeSettings) {
	perms, _ := doc["permissions"].(map[string]any)
	if perms == nil {
		perms = map[string]any{}
		doc["permissions"] = perms
	}

	// Build set of existing allow entries.
	existing := map[string]bool{}
	existingList, _ := perms["allow"].([]any)
	for _, v := range existingList {
		if s, ok := v.(string); ok {
			existing[s] = true
		}
	}

	// Append any configured entries that aren't already present.
	for _, entry := range cs.Allow {
		if !existing[entry] {
			existingList = append(existingList, entry)
		}
	}
	perms["allow"] = existingList

	// Set defaultMode only if the key is absent (preserve user's value).
	if _, ok := perms["defaultMode"]; !ok {
		mode := cs.DefaultMode
		if mode == "" {
			mode = defaultMode
		}
		perms["defaultMode"] = mode
	}
}
