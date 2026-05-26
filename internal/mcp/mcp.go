// Package mcp writes mcpServers blocks into .mcp.json files that Claude Code reads.
package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/talkersoft/hive-deck/internal/config"
)

const mcpFile = ".mcp.json"

type mcpServer struct {
	Command string            `json:"command"`
	Args    []string          `json:"args,omitempty"`
	Env     map[string]string `json:"env,omitempty"`
}

// Apply writes an mcpServers block into {decksRoot}/{deckName}/.mcp.json,
// and when deck.enableRootMCP is true (the default), merges those servers into
// {decksRoot}/.mcp.json as well.
// No-op when mcp_manager.enabled is false or the deck lists no registries.
func Apply(decksRoot string, l *config.Loaded) error {
	if !l.Setup.MCPManager.Enabled {
		return nil
	}
	if len(l.DeckFile.MCPs.Registries) == 0 {
		return nil
	}

	servers := make(map[string]*mcpServer)
	for _, regName := range l.DeckFile.MCPs.Registries {
		reg, ok := l.MCPDefs[regName]
		if !ok {
			return fmt.Errorf("mcp registry %q listed in deck but not defined in mcps.yaml", regName)
		}
		for name, def := range reg.Servers {
			resolved := make([]string, len(def.Args))
			for i, arg := range def.Args {
				resolved[i] = resolveArg(decksRoot, arg)
			}
			servers[name] = &mcpServer{
				Command: def.Command,
				Args:    resolved,
				Env:     def.Env,
			}
		}
	}

	if len(servers) == 0 {
		return nil
	}

	deckFile := filepath.Join(decksRoot, l.DeckName, mcpFile)
	if err := writeFile(deckFile, servers, false); err != nil {
		return err
	}
	fmt.Printf("mcp: wrote %d server(s) to %s\n", len(servers), deckFile)

	if l.Setup.Deck.RootMCPEnabled() {
		rootFile := filepath.Join(decksRoot, mcpFile)
		if err := writeFile(rootFile, servers, true); err != nil {
			return err
		}
		fmt.Printf("mcp: merged %d server(s) into %s\n", len(servers), rootFile)
	}

	return nil
}

// resolveArg resolves a single MCP arg to its final value.
// Absolute paths are returned unchanged. npm scoped packages (@scope/pkg)
// and flags (--flag) are not file paths and are returned as-is.
// Relative paths are joined with decksRoot.
func resolveArg(decksRoot, arg string) string {
	if filepath.IsAbs(arg) {
		return arg
	}
	if len(arg) > 0 && (arg[0] == '@' || arg[0] == '-') {
		return arg
	}
	return filepath.Join(decksRoot, arg)
}

// writeFile writes servers into a .mcp.json file.
// When merge is true, existing mcpServers entries are preserved (accumulated from other decks).
// When merge is false, the mcpServers block is replaced wholesale.
func writeFile(path string, servers map[string]*mcpServer, merge bool) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}

	doc := map[string]any{}
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, &doc); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("read %s: %w", path, err)
	}

	if merge {
		existing, _ := doc["mcpServers"].(map[string]any)
		if existing == nil {
			existing = map[string]any{}
		}
		for name, srv := range servers {
			existing[name] = srv
		}
		doc["mcpServers"] = existing
	} else {
		doc["mcpServers"] = servers
	}

	b, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(path, b, 0o644)
}
