// Package config loads and validates the active deck file and ~/.hv/config.yaml.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

const (
	ConfigDir            = ".hv"
	SetupFile            = "config.yaml"
	ModulesFile          = "modules.yaml"
	MCPsFile             = "mcps.yaml"
	ClaudeProfilesFile   = "claude-profiles.yaml"
	GitignoreRulesetsFile = "gitignore-rulesets.yaml"
	LaunchDir            = "hive-workspace"
	DecksDir             = "decks"
	WorkflowsFile        = "workflows.yaml"
	WorkflowsDir         = "workflows"
)

// MCPConfig holds the MCP subscription list for a deck.
type MCPConfig struct {
	Registries []string `yaml:"registries"`
}

// MCPRegistry is a named group of MCP server definitions in mcps.yaml.
type MCPRegistry struct {
	Servers map[string]MCPDefinition `yaml:"servers"`
}

// DeckFile is the parsed content of a deck YAML file (e.g. cloud-manager.yaml).
// The filename stem determines the deck folder name; deck: is the folder tree.
type DeckFile struct {
	Branch        string    `yaml:"branch"` // target branch for this deck; "" means use git's remote default
	Agent         string    `yaml:"agent"` // e.g. "claude" or "codex"; controls fragment output
	ClaudeProfile string    `yaml:"claude_profile"` // named profile from claude-profiles.yaml; "" skips deck-root write
	MCPs          MCPConfig `yaml:"mcps"`
	Deck          TreeNode  `yaml:"deck"`
}

// TreeNode represents a folder in the deck tree. All keys are optional and
// can coexist on the same node.
type TreeNode struct {
	RepoRefs        []string            // repos: [org/repo, ...]
	ModuleRefs      []string            // modules: [name, ...]
	Symlinks        []string            // symlinks: [target, ...]
	ShowInWorkspace bool                // show_in_workspace: true (VS Code sidebar)
	GitignoreRuleset string             // gitignore_ruleset: <name> — overrides global default for this node and its descendants
	Children        map[string]TreeNode // all other keys become subfolders
}

// UnmarshalYAML separates reserved keys from child folder keys.
func (n *TreeNode) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.MappingNode {
		return fmt.Errorf("tree node must be a mapping, got kind=%v", value.Kind)
	}
	n.Children = make(map[string]TreeNode)
	for i := 0; i+1 < len(value.Content); i += 2 {
		key := value.Content[i].Value
		val := value.Content[i+1]
		switch key {
		case "repos":
			if err := val.Decode(&n.RepoRefs); err != nil {
				return fmt.Errorf("repos: %w", err)
			}
		case "modules":
			if err := val.Decode(&n.ModuleRefs); err != nil {
				return fmt.Errorf("modules: %w", err)
			}
		case "symlinks":
			if err := val.Decode(&n.Symlinks); err != nil {
				return fmt.Errorf("symlinks: %w", err)
			}
		case "show_in_workspace":
			if err := val.Decode(&n.ShowInWorkspace); err != nil {
				return fmt.Errorf("show_in_workspace: %w", err)
			}
		case "gitignore_ruleset":
			if err := val.Decode(&n.GitignoreRuleset); err != nil {
				return fmt.Errorf("gitignore_ruleset: %w", err)
			}
		default:
			var child TreeNode
			if err := val.Decode(&child); err != nil {
				return fmt.Errorf("child %q: %w", key, err)
			}
			n.Children[key] = child
		}
	}
	return nil
}

type Module struct {
	Org   string   `yaml:"org"`
	Repos []string `yaml:"repos"`
}

// WorkspaceConfig holds workspace-root-level settings.
type WorkspaceConfig struct {
	Root          string `yaml:"root"`
	Profile       string `yaml:"profile"`
	EnableRootMCP *bool  `yaml:"enableRootMCP"`
}

// RootMCPEnabled returns true unless enableRootMCP is explicitly set to false.
func (w WorkspaceConfig) RootMCPEnabled() bool {
	return w.EnableRootMCP == nil || *w.EnableRootMCP
}

type Setup struct {
	Workspace         WorkspaceConfig            `yaml:"workspace"`
	Orgs              map[string]Org             `yaml:"orgs"`
	Gitignore         GitignoreConfig            `yaml:"gitignore"`
	GitignoreRulesets map[string]GitignoreConfig `yaml:"gitignore_rulesets"`
	Readme            ReadmeConfig               `yaml:"readme"`
	Ship              ShipConfig                 `yaml:"ship"`
	MCPManager        MCPManagerConfig           `yaml:"mcp_manager"`
	PlanFolder        string                     `yaml:"plan_folder"`
	ExecFolder        string                     `yaml:"exec_folder"`
}

// PRMode controls how hv ship handles pull requests after opening them.
type PRMode string

const (
	// PRModeAutoMerge enables GitHub auto-merge on each PR and transitions
	// to the next branch immediately. This is the default.
	PRModeAutoMerge PRMode = "auto_merge"
	// PRModeAwaitMerge opens PRs without auto-merge and polls until all are
	// merged before transitioning. Use hv_await_merge (MCP) to drive this.
	PRModeAwaitMerge PRMode = "await_merge"
	// PRModeManual opens PRs and stops. The user merges and runs hv next.
	PRModeManual PRMode = "manual"
)

// ShipConfig controls behaviour of `hv ship`.
type ShipConfig struct {
	PRMode              PRMode        `yaml:"pr_mode"`
	DeleteBranchOnMerge bool          `yaml:"delete_branch_on_merge"`
	TeardownOnShip      bool          `yaml:"teardown_on_ship"`
	MergePollInterval   time.Duration `yaml:"merge_poll_interval"`
	MergePollTimeout    time.Duration `yaml:"merge_poll_timeout"`
	OpenBrowser         bool          `yaml:"open_browser"`
}

// UnmarshalYAML implements backward-compatible parsing so configs that still
// use the old auto_merge / require_merged_pr booleans continue to work.
func (s *ShipConfig) UnmarshalYAML(value *yaml.Node) error {
	// Decode into a raw map to inspect all keys before committing.
	raw := map[string]yaml.Node{}
	if err := value.Decode(&raw); err != nil {
		return err
	}

	// Handle the fields we control directly.
	if n, ok := raw["delete_branch_on_merge"]; ok {
		n.Decode(&s.DeleteBranchOnMerge)
	}
	if n, ok := raw["open_browser"]; ok {
		n.Decode(&s.OpenBrowser)
	}
	if n, ok := raw["teardown_on_ship"]; ok {
		n.Decode(&s.TeardownOnShip)
	}
	if n, ok := raw["open_browser"]; ok {
		n.Decode(&s.OpenBrowser)
	}
	if n, ok := raw["merge_poll_interval"]; ok {
		var d string
		if err := n.Decode(&d); err == nil {
			parsed, err := time.ParseDuration(d)
			if err == nil {
				s.MergePollInterval = parsed
			}
		}
	}
	if n, ok := raw["merge_poll_timeout"]; ok {
		var d string
		if err := n.Decode(&d); err == nil {
			parsed, err := time.ParseDuration(d)
			if err == nil {
				s.MergePollTimeout = parsed
			}
		}
	}

	// pr_mode takes precedence. If present and non-empty, use it.
	if n, ok := raw["pr_mode"]; ok {
		var mode PRMode
		if err := n.Decode(&mode); err == nil && mode != "" {
			s.PRMode = mode
			return nil
		}
	}

	// Backward compat: derive pr_mode from legacy boolean fields.
	var autoMerge, requireMergedPR bool
	if n, ok := raw["auto_merge"]; ok {
		n.Decode(&autoMerge)
	}
	if n, ok := raw["require_merged_pr"]; ok {
		n.Decode(&requireMergedPR)
	}
	switch {
	case autoMerge:
		s.PRMode = PRModeAutoMerge
	case requireMergedPR:
		s.PRMode = PRModeManual
	default:
		s.PRMode = PRModeAutoMerge
	}
	return nil
}

// MCPManagerConfig controls MCP server config writing at init time.
type MCPManagerConfig struct {
	Enabled     bool   `yaml:"enabled"`
	RootMCPMode string `yaml:"root_mcp_mode"` // "overwrite" | "merge"; default "overwrite"
}

// MCPDefinition is a single MCP server entry from mcps.yaml.
// Relative Args entries are resolved against decks_root at apply time.
type MCPDefinition struct {
	Command string            `yaml:"command"`
	Args    []string          `yaml:"args"`
	Env     map[string]string `yaml:"env"`
}

// GitignoreConfig lists patterns written to a provisioned folder's .gitignore.
// Applied idempotently — existing lines are never duplicated.
type GitignoreConfig struct {
	Entries []string `yaml:"entries"`
}

// ReadmeConfig controls README.md generation at the deck and folder level.
// Existing README.md files are never overwritten.
type ReadmeConfig struct {
	Enabled bool `yaml:"enabled"`
}

type Org struct {
	URL      string `yaml:"url"`
	Protocol string `yaml:"protocol"`
}

// ClaudePermissions mirrors Claude Code's permissions schema exactly so the
// YAML subtree serialises directly to/from settings.local.json with no translation.
type ClaudePermissions struct {
	Allow       []string `yaml:"allow"                 json:"allow"`
	Deny        []string `yaml:"deny,omitempty"        json:"deny,omitempty"`
	DefaultMode string   `yaml:"defaultMode,omitempty" json:"defaultMode,omitempty"`
}

// ClaudeSettings, when Enabled, causes hv to write settings.local.json into
// each cloned repo's .claude/ directory. Mode controls write behaviour:
// "overwrite" (default) always replaces the file, "skip" is a no-op when the
// file exists, "merge" unions the allow lists and preserves existing defaultMode.
type ClaudeSettings struct {
	Enabled        bool              `yaml:"enabled"`
	Mode           string            `yaml:"mode"`            // "overwrite" | "skip" | "merge"; default "overwrite"
	ForceOverwrite *bool             `yaml:"force_overwrite"` // deprecated; use mode instead
	Permissions    ClaudePermissions `yaml:"permissions"`
	DefaultMode    string            `yaml:"defaultMode"`
}

// Loaded bundles the deck file, setup, modules, home directory, deck name
// (derived from filename stem), and the absolute path of the deck file.
type Loaded struct {
	Home          string
	DeckFile      DeckFile
	DeckName      string                     // filename stem, e.g. "cloud-manager" from "cloud-manager.yaml"
	Modules       map[string]Module          // from modules.yaml
	MCPDefs       map[string]MCPRegistry     // from mcps.yaml; empty map when file is absent
	ClaudeProfiles map[string]ClaudeSettings // from claude-profiles.yaml; empty map when file is absent
	Setup         Setup
	DeckPath      string
}

// LoadSetup returns the hv home and parsed setup.yaml without loading any
// deck file. Intended for commands that list decks.
func LoadSetup() (string, Setup, error) {
	home, err := FindHome()
	if err != nil {
		return "", Setup{}, err
	}
	p, err := findConfigFile(SetupFile)
	if err != nil || p == "" {
		return "", Setup{}, fmt.Errorf("%s not found — set $HV_HOME to point at your workflow-configuration repo", SetupFile)
	}
	setup, err := loadSetup(p)
	if err != nil {
		return "", Setup{}, err
	}
	return home, setup, nil
}

// LoadDeck loads config.yaml + modules.yaml + claude-profiles.yaml +
// gitignore-rulesets.yaml + the deck file for the given name.
// name may be "cloud-manager" or "cloud-manager.yaml" — both are accepted.
func LoadDeck(name string) (*Loaded, error) {
	home, setup, err := LoadSetup()
	if err != nil {
		return nil, err
	}
	deckPath, deckName, err := resolveDeckPath(name)
	if err != nil {
		return nil, err
	}
	df, err := loadDeckFile(deckPath)
	if err != nil {
		return nil, err
	}
	mods, err := LoadModules()
	if err != nil {
		return nil, err
	}
	mcpDefs, err := LoadMCPs()
	if err != nil {
		return nil, err
	}
	claudeProfiles, err := LoadClaudeProfiles()
	if err != nil {
		return nil, err
	}
	gitignoreRulesets, err := LoadGitignoreRulesets()
	if err != nil {
		return nil, err
	}
	if len(gitignoreRulesets) > 0 {
		if setup.GitignoreRulesets == nil {
			setup.GitignoreRulesets = make(map[string]GitignoreConfig)
		}
		for k, v := range gitignoreRulesets {
			setup.GitignoreRulesets[k] = v
		}
	}
	return &Loaded{
		Home:           home,
		DeckFile:       df,
		DeckName:       deckName,
		Modules:        mods,
		MCPDefs:        mcpDefs,
		ClaudeProfiles: claudeProfiles,
		Setup:          setup,
		DeckPath:       deckPath,
	}, nil
}

// LoadClaudeProfiles reads claude-profiles.yaml using the CWD-first search order.
// Returns an empty map (not an error) if the file is absent.
func LoadClaudeProfiles() (map[string]ClaudeSettings, error) {
	path, err := findConfigFile(ClaudeProfilesFile)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return make(map[string]ClaudeSettings), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var profiles map[string]ClaudeSettings
	if err := yaml.Unmarshal(b, &profiles); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if profiles == nil {
		profiles = make(map[string]ClaudeSettings)
	}
	return profiles, nil
}

// LoadGitignoreRulesets reads gitignore-rulesets.yaml using the CWD-first search order.
// Returns an empty map (not an error) if the file is absent.
func LoadGitignoreRulesets() (map[string]GitignoreConfig, error) {
	path, err := findConfigFile(GitignoreRulesetsFile)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return make(map[string]GitignoreConfig), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var rulesets map[string]GitignoreConfig
	if err := yaml.Unmarshal(b, &rulesets); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if rulesets == nil {
		rulesets = make(map[string]GitignoreConfig)
	}
	return rulesets, nil
}

// LoadMCPs reads mcps.yaml using the CWD-first search order.
// Returns an empty map (not an error) if the file is absent.
func LoadMCPs() (map[string]MCPRegistry, error) {
	path, err := findConfigFile(MCPsFile)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return make(map[string]MCPRegistry), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var defs map[string]MCPRegistry
	if err := yaml.Unmarshal(b, &defs); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if defs == nil {
		defs = make(map[string]MCPRegistry)
	}
	return defs, nil
}

// FindModulesPath returns the absolute path of modules.yaml using the
// CWD-first search order. Returns ("", nil) if not found.
func FindModulesPath() (string, error) {
	return findConfigFile(ModulesFile)
}

// LoadModules reads modules.yaml using the CWD-first search order.
// Returns an empty map (not an error) if the file is absent — decks that
// use only repos: can omit modules.yaml entirely.
func LoadModules() (map[string]Module, error) {
	path, err := findConfigFile(ModulesFile)
	if err != nil {
		return nil, err
	}
	if path == "" {
		return make(map[string]Module), nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var mods map[string]Module
	if err := yaml.Unmarshal(b, &mods); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if mods == nil {
		mods = make(map[string]Module)
	}
	return mods, nil
}

// fileExists returns true if the path exists and is accessible.
func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// findInConfigSubdir looks for <subdir>/<filename> inside a .hv/ directory
// using the same search order as findConfigFile:
//
//  1. $HV_HOME/<subdir>/<filename>      — direct path (workflow-configuration repo)
//  1b.$HV_HOME/.hv/<subdir>/<filename>  — legacy .hv/ append (errors if neither found)
//  2. <CWD>/.hv/<subdir>/<filename>     — project-local wins when present
//  3. $HOME/.hv/<subdir>/<filename>     — global user install fallback
//
// Returns ("", nil) when not found in locations 2–3.
func findInConfigSubdir(subdir, filename string) (string, error) {
	if h := os.Getenv("HV_HOME"); h != "" {
		// Try direct path first (repo root IS the config dir).
		if p := filepath.Join(h, subdir, filename); fileExists(p) {
			return p, nil
		}
		// Fall back to legacy .hv/ append.
		p := filepath.Join(h, ConfigDir, subdir, filename)
		if !fileExists(p) {
			return "", fmt.Errorf("HV_HOME=%q is set but %s not found (tried direct and .hv/ paths)", h, filepath.Join(h, subdir, filename))
		}
		return p, nil
	}
	if cwd, err := os.Getwd(); err == nil {
		p := filepath.Join(cwd, ConfigDir, subdir, filename)
		if fileExists(p) {
			return p, nil
		}
	}
	if h, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(h, ConfigDir, subdir, filename)
		if fileExists(p) {
			return p, nil
		}
	}
	return "", nil
}

// resolveDeckPath resolves a deck name to its absolute path and stem using
// the CWD-first search order into .hv/decks/. Accepts "cloud-manager" or
// "cloud-manager.yaml".
func resolveDeckPath(name string) (path, stem string, err error) {
	if name == "" {
		return "", "", fmt.Errorf("no deck specified")
	}
	if strings.ContainsAny(name, "/\\") || strings.Contains(name, "..") {
		return "", "", fmt.Errorf("deck name %q must be a bare name (no path components)", name)
	}
	filename := name
	if !strings.HasSuffix(filename, ".yaml") {
		filename += ".yaml"
	}
	p, err := findInConfigSubdir(DecksDir, filename)
	if err != nil {
		return "", "", err
	}
	if p == "" {
		return "", "", fmt.Errorf("deck %q not found — looked for %s in .hv/decks/ under CWD then $HOME", name, filename)
	}
	s := strings.TrimSuffix(filename, ".yaml")
	return p, s, nil
}

// FindHome locates the hv home directory — the directory that contains
// the .hive/ folder — using the CWD-first search order.
func FindHome() (string, error) {
	p, err := findConfigFile(SetupFile)
	if err != nil {
		return "", err
	}
	if p == "" {
		h, _ := os.UserHomeDir()
		return "", fmt.Errorf("%s/.hv/%s not found — run 'make install' from the hive-deck checkout to populate it (or set $HV_HOME to point at a directory containing .hv/)", h, SetupFile)
	}
	// <home>/.hv/config.yaml → strip filename → strip .hv/ → home
	return filepath.Dir(filepath.Dir(p)), nil
}

// FindDecksDir returns the absolute path of the decks/ directory using the
// same search order as findInConfigSubdir.
func FindDecksDir() (string, error) {
	if h := os.Getenv("HV_HOME"); h != "" {
		if d := filepath.Join(h, DecksDir); fileExists(d) {
			return d, nil
		}
		d := filepath.Join(h, ConfigDir, DecksDir)
		if !fileExists(d) {
			return "", fmt.Errorf("HV_HOME=%q is set but decks/ directory not found (tried %s and %s)", h, filepath.Join(h, DecksDir), d)
		}
		return d, nil
	}
	if cwd, err := os.Getwd(); err == nil {
		if d := filepath.Join(cwd, ConfigDir, DecksDir); fileExists(d) {
			return d, nil
		}
	}
	if h, err := os.UserHomeDir(); err == nil {
		if d := filepath.Join(h, ConfigDir, DecksDir); fileExists(d) {
			return d, nil
		}
	}
	return "", nil
}

// findConfigFile looks for <filename> inside a .hv/ directory using this
// search order:
//
//  1. $HV_HOME/<filename>      — direct path (workflow-configuration repo as root)
//  1b.$HV_HOME/.hv/<filename>  — legacy .hv/ append (errors if neither found)
//  2. <CWD>/.hv/<filename>     — project-local config wins when present
//  3. $HOME/.hv/<filename>     — global user install fallback
//
// Returns ("", nil) when the file is not found in any location (not an error
// by itself — callers decide how to handle absence). Returns a non-nil error
// only when $HV_HOME is set but the file is not there.
func findConfigFile(filename string) (string, error) {
	if h := os.Getenv("HV_HOME"); h != "" {
		// Try direct path first (repo root IS the config dir).
		if p := filepath.Join(h, filename); fileExists(p) {
			return p, nil
		}
		// Fall back to legacy .hv/ append.
		p := filepath.Join(h, ConfigDir, filename)
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("HV_HOME=%q is set but %s not found (tried direct and .hv/ paths)", h, filepath.Join(h, filename))
		}
		return p, nil
	}
	if cwd, err := os.Getwd(); err == nil {
		p := filepath.Join(cwd, ConfigDir, filename)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	if h, err := os.UserHomeDir(); err == nil {
		p := filepath.Join(h, ConfigDir, filename)
		if _, err := os.Stat(p); err == nil {
			return p, nil
		}
	}
	return "", nil
}

func loadDeckFile(path string) (DeckFile, error) {
	var df DeckFile
	b, err := os.ReadFile(path)
	if err != nil {
		return df, fmt.Errorf("read %s: %w", path, err)
	}
	// Detect old format: presence of top-level "workspace:" or "wksp:" key.
	var probe map[string]any
	if probeErr := yaml.Unmarshal(b, &probe); probeErr == nil {
		if _, hasOld := probe["workspace"]; hasOld {
			return df, fmt.Errorf("old format in %s — replace workspace:/system:/project: with a deck: tree", path)
		}
		if _, hasOld := probe["wksp"]; hasOld {
			return df, fmt.Errorf("old format in %s — rename wksp: to deck:", path)
		}
	}
	if err := yaml.Unmarshal(b, &df); err != nil {
		return df, fmt.Errorf("parse %s: %w", path, err)
	}
	return df, nil
}

func loadSetup(path string) (Setup, error) {
	var s Setup
	b, err := os.ReadFile(path)
	if err != nil {
		return s, fmt.Errorf("read %s: %w (run 'make setup' to scaffold from .hv/config.yaml.example)", path, err)
	}
	if err := yaml.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("parse %s: %w", path, err)
	}
	return s, nil
}

// ExpandRoot resolves a leading ~ to the user's home directory.
func ExpandRoot(root string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("workspace.root is empty")
	}
	if root == "~" || (len(root) >= 2 && root[:2] == "~/") {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if root == "~" {
			return h, nil
		}
		return filepath.Join(h, root[2:]), nil
	}
	return root, nil
}
