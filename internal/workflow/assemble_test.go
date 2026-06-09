package workflow

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/talkersoft/hive-deck/internal/config"
)

func TestAssemble_Shorthand(t *testing.T) {
	def := &config.WorkflowDef{
		Tasks: config.JobBody{
			Jobs: []string{"setup"},
		},
	}
	jobs := map[string]config.JobDef{
		"setup": {Fragments: []string{"@status-check", "@folder-structure"}},
	}
	got, err := Assemble(def, jobs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"@status-check", "@folder-structure"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestAssemble_LongForm(t *testing.T) {
	def := &config.WorkflowDef{
		Tasks: config.JobBody{
			Init: []string{"@init-frag"},
			Jobs: []string{"outer"},
			Post: []string{"@post-frag"},
		},
	}
	jobs := map[string]config.JobDef{
		"outer": {
			Init: []string{"@outer-init"},
			Jobs: []string{"inner"},
			Post: []string{"@outer-post"},
		},
		"inner": {Fragments: []string{"@inner-frag"}},
	}
	got, err := Assemble(def, jobs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"@init-frag", "@outer-init", "@inner-frag", "@outer-post", "@post-frag"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// createWorkflowJobs returns the job map that models the hive-deck create-workflow workflow.
// Depth-first expansion must yield exactly the expected slice in TC-004.
func createWorkflowJobs() map[string]config.JobDef {
	return map[string]config.JobDef{
		"write-plans": {
			Init: []string{"@write-init"},
			Jobs: []string{"write-deck-orch", "write-tasks"},
			Post: []string{},
		},
		"write-deck-orch": {Fragments: []string{"@write-deck", "@write-orch"}},
		"write-tasks": {
			Init: []string{"@write-task-0000"},
			Jobs: []string{"write-impl"},
			Post: []string{"@write-test-0000", "@write-test"},
		},
		"write-impl": {Fragments: []string{"@write-task", "@write-fix"}},
		"handoff":    {Fragments: []string{"@write-result-lessons", "@ship-plans", "@key-rules"}},
	}
}

func TestAssemble_CreateWorkflow(t *testing.T) {
	def := &config.WorkflowDef{
		Tasks: config.JobBody{
			Init: []string{"@status-check"},
			Jobs: []string{"write-plans", "handoff"},
			Post: []string{},
		},
	}
	jobs := createWorkflowJobs()
	got, err := Assemble(def, jobs, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		"@status-check",
		"@write-init",
		"@write-deck",
		"@write-orch",
		"@write-task-0000",
		"@write-task",
		"@write-fix",
		"@write-test-0000",
		"@write-test",
		"@write-result-lessons",
		"@ship-plans",
		"@key-rules",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %v\nwant %v", got, want)
	}
}

func TestAssemble_MissingJob(t *testing.T) {
	def := &config.WorkflowDef{
		Tasks: config.JobBody{
			Jobs: []string{"no-such-job"},
		},
	}
	_, err := Assemble(def, map[string]config.JobDef{}, nil)
	if err == nil {
		t.Fatal("expected error for missing job, got nil")
	}
}

func TestAssemble_CircularRef(t *testing.T) {
	def := &config.WorkflowDef{
		Tasks: config.JobBody{
			Jobs: []string{"a"},
		},
	}
	jobs := map[string]config.JobDef{
		"a": {Jobs: []string{"b"}},
		"b": {Jobs: []string{"a"}},
	}
	_, err := Assemble(def, jobs, nil)
	if err == nil {
		t.Fatal("expected cycle error, got nil")
	}
}

func TestRender_Tokens(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "fragments"), 0755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "fragments", "hello.md"), []byte("deck={{.Deck}} folder={{.ExecFolder}}\n"), 0644)

	data := TemplateData{
		Deck:       "my-deck",
		ExecFolder: "/tmp/wf",
	}
	result, err := Render(root, []string{"@hello"}, data, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "deck=my-deck folder=/tmp/wf\n"
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}
