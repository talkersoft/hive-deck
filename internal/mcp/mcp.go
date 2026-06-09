// Package mcp writes mcpServers blocks into .mcp.json files that Claude Code reads.
package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
		deckRoot := filepath.Join(decksRoot, l.DeckName)
		for name, def := range reg.Servers {
			resolved := make([]string, len(def.Args))
			for i, arg := range def.Args {
				resolved[i] = resolveArg(decksRoot, deckRoot, arg)
			}
			resolvedEnv := make(map[string]string, len(def.Env))
			for k, v := range def.Env {
				resolvedEnv[k] = resolveEnvVal(deckRoot, decksRoot, v)
			}
			servers[name] = &mcpServer{
				Command: def.Command,
				Args:    resolved,
				Env:     resolvedEnv,
			}
		}
	}

	if len(servers) == 0 {
		return nil
	}

	if l.Setup.Workspace.RootMCPEnabled() {
		rootFile := filepath.Join(decksRoot, mcpFile)
		mergeRoot := l.Setup.MCPManager.RootMCPMode == "merge"
		if err := writeFile(rootFile, servers, mergeRoot); err != nil {
			return err
		}
		action := "wrote"
		if mergeRoot {
			action = "merged"
		}
		fmt.Printf("mcp: %s %d server(s) into %s\n", action, len(servers), rootFile)
	}

	return nil
}

// resolveArg resolves a single MCP arg to its final value.
// {{DecksRoot}} and {{DeckRoot}} tokens are expanded first.
// Absolute paths are returned unchanged after expansion. npm scoped packages
// (@scope/pkg) and flags (--flag) are not file paths and are returned as-is.
// Relative paths are joined with decksRoot.
func resolveArg(decksRoot, deckRoot, arg string) string {
	arg = strings.ReplaceAll(arg, "{{DecksRoot}}", decksRoot)
	arg = strings.ReplaceAll(arg, "{{DeckRoot}}", deckRoot)
	if filepath.IsAbs(arg) {
		return arg
	}
	if len(arg) > 0 && (arg[0] == '@' || arg[0] == '-') {
		return arg
	}
	return filepath.Join(decksRoot, arg)
}

// resolveEnvVal expands {{DeckRoot}} and {{DecksRoot}} in env var values.
// {{DeckRoot}}  = decksRoot/deckName  (e.g. ~/workspace/hive-deck-pro)
// {{DecksRoot}} = decksRoot           (e.g. ~/workspace) — deck-independent
func resolveEnvVal(deckRoot, decksRoot, val string) string {
	val = strings.ReplaceAll(val, "{{DeckRoot}}", deckRoot)
	val = strings.ReplaceAll(val, "{{DecksRoot}}", decksRoot)
	return val
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
