package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// WorkflowDef is loaded from workflows/<deck>/<name>.yaml in the workflow_root.
type WorkflowDef struct {
	Repo   string            `yaml:"repo"`
	Tokens map[string]string `yaml:"tokens"`
	Tasks  JobBody           `yaml:"tasks"`
}

// JobDef is loaded from jobs/<group>/<name>.yaml in the workflow_root.
// Fragments and Init/Jobs/Post are mutually exclusive.
type JobDef struct {
	Description string   `yaml:"description"`
	Fragments   []string `yaml:"fragments"`
	Init        []string `yaml:"init"`
	Jobs        []string `yaml:"jobs"`
	Post        []string `yaml:"post"`
}

// JobBody is the shared init/jobs/post shape used by WorkflowDef.Tasks and JobDef long form.
type JobBody struct {
	Init []string `yaml:"init"`
	Jobs []string `yaml:"jobs"`
	Post []string `yaml:"post"`
}

// LoadWorkflowDef loads <root>/workflows/<deck>/<name>.yaml, falling back to
// <root>/workflows/shared/<name>.yaml when no deck-specific file exists.
func LoadWorkflowDef(root, deck, name string) (*WorkflowDef, error) {
	candidates := []string{
		filepath.Join(root, "workflows", deck, name+".yaml"),
		filepath.Join(root, "workflows", "shared", name+".yaml"),
	}
	for _, p := range candidates {
		b, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read workflow %q: %w", p, err)
		}
		var def WorkflowDef
		if err := yaml.Unmarshal(b, &def); err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}
		return &def, nil
	}
	return nil, fmt.Errorf("workflow %q not found for deck %q (checked deck-specific and shared/)", name, deck)
}

// LoadAllJobs walks <root>/jobs/**/*.yaml and returns a map keyed by file stem.
// Returns an error on duplicate stems, mixed fragments/init usage, or missing @ prefix.
func LoadAllJobs(root string) (map[string]JobDef, error) {
	jobsDir := filepath.Join(root, "jobs")
	jobs := make(map[string]JobDef)

	err := filepath.WalkDir(jobsDir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) {
				return filepath.SkipAll
			}
			return walkErr
		}
		if d.IsDir() || !strings.HasSuffix(path, ".yaml") {
			return nil
		}
		stem := strings.TrimSuffix(d.Name(), ".yaml")
		if _, exists := jobs[stem]; exists {
			return fmt.Errorf("duplicate job name %q — stems must be unique across all groups", stem)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read job %s: %w", path, err)
		}
		var def JobDef
		if err := yaml.Unmarshal(b, &def); err != nil {
			return fmt.Errorf("parse job %s: %w", path, err)
		}
		if err := validateJobDef(stem, def); err != nil {
			return err
		}
		jobs[stem] = def
		return nil
	})
	if err != nil {
		return nil, err
	}
	return jobs, nil
}

// validateJobDef enforces mutual exclusion and @ prefix rules.
func validateJobDef(name string, def JobDef) error {
	hasShorthand := len(def.Fragments) > 0
	hasLongForm := len(def.Init) > 0 || len(def.Jobs) > 0 || len(def.Post) > 0
	if hasShorthand && hasLongForm {
		return fmt.Errorf("job %q uses both fragments: and init:/jobs:/post: — pick one form", name)
	}
	for _, ref := range def.Fragments {
		if !strings.HasPrefix(ref, "@") {
			return fmt.Errorf("job %q: fragment ref %q is missing @ prefix", name, ref)
		}
	}
	for _, ref := range def.Init {
		if !strings.HasPrefix(ref, "@") {
			return fmt.Errorf("job %q: init ref %q is missing @ prefix", name, ref)
		}
	}
	for _, ref := range def.Post {
		if !strings.HasPrefix(ref, "@") {
			return fmt.Errorf("job %q: post ref %q is missing @ prefix", name, ref)
		}
	}
	return nil
}

// LoadWorkflowExtension reads ~/.hv/workflows/<type>/<name>.yaml.
func LoadWorkflowExtension(wfType, name string) ([]byte, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	p := filepath.Join(home, ".hv", "workflows", wfType, name+".yaml")
	b, err := os.ReadFile(p)
	if err != nil {
		return nil, fmt.Errorf("workflow extension %q not found at %s", name, p)
	}
	return b, nil
}

// LoadFragmentWithFallback reads a fragment using a bubble-up search order:
//  1. decks/workflows/<type>/<deck>/fragments/<name>.md  — deck-scoped (skipped when deck is "")
//  2. decks/workflows/<type>/fragments/<name>.md         — shared across all decks
//  3. builtin FS                                         — system fallback
func LoadFragmentWithFallback(root, name, deck string, fallbackFS fs.FS) (string, error) {
	name = strings.TrimPrefix(name, "@")
	for _, wfType := range []string{"workflow", "plan"} {
		if deck != "" {
			p := filepath.Join(root, "decks", "workflows", wfType, deck, "fragments", name+".md")
			if b, err := os.ReadFile(p); err == nil {
				return string(b), nil
			}
		}
		p := filepath.Join(root, "decks", "workflows", wfType, "fragments", name+".md")
		if b, err := os.ReadFile(p); err == nil {
			return string(b), nil
		}
	}
	if fallbackFS != nil {
		for _, sub := range []string{"workflow/fragments", "plan/fragments"} {
			if fb, ferr := fs.ReadFile(fallbackFS, sub+"/"+name+".md"); ferr == nil {
				return string(fb), nil
			}
		}
	}
	return "", fmt.Errorf("fragment %q not found for deck %q in %s or builtin", name, deck, filepath.Join(root, "decks/workflows"))
}

// DetectCycles performs a DFS over the job graph and returns an error naming
// the cycle if one is found.
func DetectCycles(jobs map[string]JobDef) error {
	const (
		unvisited = 0
		visiting  = 1
		visited   = 2
	)
	state := make(map[string]int, len(jobs))
	path := make([]string, 0)

	var dfs func(name string) error
	dfs = func(name string) error {
		switch state[name] {
		case visited:
			return nil
		case visiting:
			// Find where in path the cycle starts
			for i, n := range path {
				if n == name {
					cycle := append(path[i:], name)
					return fmt.Errorf("circular job reference: %s", strings.Join(cycle, " → "))
				}
			}
			return fmt.Errorf("circular job reference involving %q", name)
		}
		state[name] = visiting
		path = append(path, name)
		def, ok := jobs[name]
		if ok {
			for _, sub := range def.Jobs {
				if err := dfs(sub); err != nil {
					return err
				}
			}
		}
		path = path[:len(path)-1]
		state[name] = visited
		return nil
	}

	for name := range jobs {
		if err := dfs(name); err != nil {
			return err
		}
	}
	return nil
}
