// Package mcp writes the mcpServers block into {decks_root}/.claude/settings.json.
package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/talkersoft/hive-deck/internal/config"
)

const (
	claudeDir    = ".claude"
	settingsFile = "settings.json"
)

type mcpServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// Apply writes an mcpServers block into {decksRoot}/.claude/settings.json,
// merging with any existing content and preserving all other top-level keys.
// No-op when mcp_manager.enabled is false, the deck lists no MCPs, or mcps.yaml is absent.
func Apply(decksRoot string, l *config.Loaded) error {
	if !l.Setup.MCPManager.Enabled {
		return nil
	}
	if len(l.DeckFile.MCPs) == 0 {
		return nil
	}

	servers := make(map[string]*mcpServer, len(l.DeckFile.MCPs))
	for _, name := range l.DeckFile.MCPs {
		def, ok := l.MCPDefs[name]
		if !ok {
			return fmt.Errorf("mcp %q listed in deck but not defined in mcps.yaml", name)
		}
		resolved := make([]string, len(def.Args))
		for i, arg := range def.Args {
			if filepath.IsAbs(arg) {
				resolved[i] = arg
			} else {
				resolved[i] = filepath.Join(decksRoot, arg)
			}
		}
		servers[name] = &mcpServer{
			Command: def.Command,
			Args:    resolved,
			Env:     def.Env,
		}
	}

	dir := filepath.Join(decksRoot, claudeDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", dir, err)
	}

	path := filepath.Join(dir, settingsFile)
	doc := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}

	doc["mcpServers"] = servers

	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	if err := os.WriteFile(path, b, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	fmt.Printf("mcp: wrote %d server(s) to %s\n", len(servers), path)
	return nil
}
