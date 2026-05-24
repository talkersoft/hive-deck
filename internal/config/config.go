// Package config loads and validates the active deck file and ~/.hv/config.yaml.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	ConfigDir   = ".hv"
	SetupFile   = "config.yaml"
	ModulesFile = "modules.yaml"
	LaunchDir   = "hive-workspace"
)

// DeckFile is the parsed content of a deck YAML file (e.g. cloud-manager.yaml).
// The filename stem determines the deck folder name; deck: is the folder tree.
type DeckFile struct {
	Deck TreeNode `yaml:"deck"`
}

// TreeNode represents a folder in the deck tree. All keys are optional and
// can coexist on the same node.
type TreeNode struct {
	RepoRefs        []string            // repos: [org/repo, ...]
	ModuleRefs      []string            // modules: [name, ...]
	Symlinks        []string            // symlinks: [target, ...]
	WorkspaceFolder bool                // workspace_folder: true (VS Code sidebar)
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
		case "workspace_folder":
			if err := val.Decode(&n.WorkspaceFolder); err != nil {
				return fmt.Errorf("workspace_folder: %w", err)
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

type Setup struct {
	DecksRoot      string            `yaml:"decks_root"`
	Orgs           map[string]Org    `yaml:"orgs"`
	DefaultBranch  string            `yaml:"default_branch"`
	Branches       map[string]string `yaml:"branches"`
	ClaudeSettings ClaudeSettings    `yaml:"claude_settings"`
	Gitignore      GitignoreConfig   `yaml:"gitignore"`
	Readme         ReadmeConfig      `yaml:"readme"`
}

// GitignoreConfig lists patterns written to every provisioned folder's .gitignore.
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

// ClaudeSettings, when Enabled, causes hv to idempotently merge
// settings.local.json into each cloned repo's .claude/ directory.
type ClaudeSettings struct {
	Enabled     bool     `yaml:"enabled"`
	Allow       []string `yaml:"allow"`
	DefaultMode string   `yaml:"default_mode"`
}

// Loaded bundles the deck file, setup, modules, home directory, deck name
// (derived from filename stem), and the absolute path of the deck file.
type Loaded struct {
	Home     string
	DeckFile DeckFile
	DeckName string // filename stem, e.g. "cloud-manager" from "cloud-manager.yaml"
	Modules  map[string]Module // from modules.yaml
	Setup    Setup
	DeckPath string
}

// LoadSetup returns the hv home and parsed setup.yaml without loading any
// deck file. Intended for commands that list decks.
func LoadSetup() (string, Setup, error) {
	home, err := FindHome()
	if err != nil {
		return "", Setup{}, err
	}
	setup, err := loadSetup(filepath.Join(home, ConfigDir, SetupFile))
	if err != nil {
		return "", Setup{}, err
	}
	return home, setup, nil
}

// LoadDeck loads setup.yaml + modules.yaml + the deck file for the given name.
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
	return &Loaded{
		Home:     home,
		DeckFile: df,
		DeckName: deckName,
		Modules:  mods,
		Setup:    setup,
		DeckPath: deckPath,
	}, nil
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

// resolveDeckPath resolves a deck name to its absolute path and stem using
// the CWD-first search order. Accepts "cloud-manager" or "cloud-manager.yaml".
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
	if filename == SetupFile || filename == ModulesFile {
		return "", "", fmt.Errorf("deck name %q is reserved", name)
	}
	p, err := findConfigFile(filename)
	if err != nil {
		return "", "", err
	}
	if p == "" {
		return "", "", fmt.Errorf("deck %q not found — looked for %s in .hv/ under CWD then $HOME", name, filename)
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

// findConfigFile looks for <filename> inside a .hv/ directory using this
// search order:
//
//  1. $HV_HOME/.hv/<filename>  — explicit dev/CI override (errors if set but missing)
//  2. <CWD>/.hv/<filename>     — project-local config wins when present
//  3. $HOME/.hv/<filename>     — global user install fallback
//
// Returns ("", nil) when the file is not found in any location (not an error
// by itself — callers decide how to handle absence). Returns a non-nil error
// only when $HV_HOME is set but the file is not there.
func findConfigFile(filename string) (string, error) {
	if h := os.Getenv("HV_HOME"); h != "" {
		p := filepath.Join(h, ConfigDir, filename)
		if _, err := os.Stat(p); err != nil {
			return "", fmt.Errorf("HV_HOME=%q is set but %s not found", h, p)
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
		return "", fmt.Errorf("decks_root is empty")
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
