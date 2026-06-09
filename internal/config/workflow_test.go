package config

import (
	"os"
	"path/filepath"
	"testing"
)

// makeRoot creates a temp workflow_root with the given job YAML files.
// files is a map of relative path → content.
func makeRoot(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestLoadAllJobs_DuplicateStem(t *testing.T) {
	root := makeRoot(t, map[string]string{
		"jobs/a/setup.yaml": "fragments:\n  - \"@status-check\"\n",
		"jobs/b/setup.yaml": "fragments:\n  - \"@folder-structure\"\n",
	})
	_, err := LoadAllJobs(root)
	if err == nil {
		t.Fatal("expected error for duplicate stem, got nil")
	}
}

func TestLoadAllJobs_MixedFragmentsInit(t *testing.T) {
	root := makeRoot(t, map[string]string{
		"jobs/planning/bad.yaml": "fragments:\n  - \"@foo\"\ninit:\n  - \"@bar\"\n",
	})
	_, err := LoadAllJobs(root)
	if err == nil {
		t.Fatal("expected error for mixed fragments+init, got nil")
	}
}

func TestLoadAllJobs_MissingAtPrefix(t *testing.T) {
	root := makeRoot(t, map[string]string{
		"jobs/planning/bad.yaml": "fragments:\n  - no-at-sign\n", // no quotes needed — testing missing @ on plain string
	})
	_, err := LoadAllJobs(root)
	if err == nil {
		t.Fatal("expected error for missing @ prefix, got nil")
	}
}

func TestLoadAllJobs_Valid(t *testing.T) {
	root := makeRoot(t, map[string]string{
		"jobs/planning/setup.yaml":       "fragments:\n  - \"@status-check\"\n  - \"@folder-structure\"\n",
		"jobs/planning/write-plans.yaml": "init:\n  - \"@write-init\"\njobs:\n  - write-tasks\npost:\n  - \"@write-result-lessons\"\n",
	})
	jobs, err := LoadAllJobs(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := jobs["setup"]; !ok {
		t.Error("expected job 'setup'")
	}
	if _, ok := jobs["write-plans"]; !ok {
		t.Error("expected job 'write-plans'")
	}
}

func TestDetectCycles_Direct(t *testing.T) {
	jobs := map[string]JobDef{
		"a": {Jobs: []string{"a"}},
	}
	if err := DetectCycles(jobs); err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

func TestDetectCycles_Indirect(t *testing.T) {
	jobs := map[string]JobDef{
		"a": {Jobs: []string{"b"}},
		"b": {Jobs: []string{"c"}},
		"c": {Jobs: []string{"a"}},
	}
	if err := DetectCycles(jobs); err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

func TestDetectCycles_Clean(t *testing.T) {
	jobs := map[string]JobDef{
		"a": {Jobs: []string{"b", "c"}},
		"b": {Fragments: []string{"@x"}},
		"c": {Fragments: []string{"@y"}},
	}
	if err := DetectCycles(jobs); err != nil {
		t.Fatalf("unexpected cycle error: %v", err)
	}
}

func TestLoadFragment_StripAt(t *testing.T) {
	root := makeRoot(t, map[string]string{
		"decks/workflows/workflow/fragments/status-check.md": "# Status Check\n",
	})
	content, err := LoadFragmentWithFallback(root, "@status-check", "", nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if content != "# Status Check\n" {
		t.Errorf("unexpected content: %q", content)
	}
}

func TestLoadFragment_NotFound(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "fragments"), 0755)
	_, err := LoadFragmentWithFallback(root, "@missing", "", nil)
	if err == nil {
		t.Fatal("expected error for missing fragment, got nil")
	}
}
