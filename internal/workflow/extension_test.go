package workflow

import "testing"

func makeBase() BuiltinBase {
	return BuiltinBase{
		Type: "workflow",
		Init: BaseSection{Fragments: []string{"@init-frag"}},
		Post: BaseSection{Fragments: []string{"@post-frag"}},
	}
}

// TestAssembleFromExtension_ThreePhases verifies that 3 user phases produce 5 tasks:
// 0000(init) + 0001,0002,0003(user) + FINAL(post).
func TestAssembleFromExtension_ThreePhases(t *testing.T) {
	base := makeBase()
	ext := &WorkflowExtension{
		Phases: []Phase{
			{Title: "Alpha", Jobs: []string{"@job-a"}},
			{Title: "Beta", Jobs: []string{"@job-b"}},
			{Title: "Gamma", Jobs: []string{"@job-c"}},
		},
	}

	aw, err := AssembleFromExtension(base, ext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(aw.Tasks) != 5 {
		t.Fatalf("expected 5 tasks, got %d: %v", len(aw.Tasks), aw.Tasks)
	}

	// Verify structure
	cases := []struct {
		number string
		source string
		title  string
	}{
		{"0000", "builtin", "Initialize"},
		{"0001", "user", "Alpha"},
		{"0002", "user", "Beta"},
		{"0003", "user", "Gamma"},
		{"FINAL", "builtin", "Results and ship"},
	}
	for i, tc := range cases {
		task := aw.Tasks[i]
		if task.Number != tc.number {
			t.Errorf("task[%d].Number = %q, want %q", i, task.Number, tc.number)
		}
		if task.Source != tc.source {
			t.Errorf("task[%d].Source = %q, want %q", i, task.Source, tc.source)
		}
		if task.Title != tc.title {
			t.Errorf("task[%d].Title = %q, want %q", i, task.Title, tc.title)
		}
	}
}

// TestAssembleFromExtension_ZeroPhases verifies that an extension with no phases
// returns an error.
func TestAssembleFromExtension_ZeroPhases(t *testing.T) {
	base := makeBase()
	ext := &WorkflowExtension{Phases: []Phase{}}

	_, err := AssembleFromExtension(base, ext, nil)
	if err == nil {
		t.Fatal("expected error for zero phases, got nil")
	}
}

// TestAssembleFromExtension_NilExt verifies that a nil extension still produces
// the two builtin tasks (init + post) without error.
func TestAssembleFromExtension_NilExt(t *testing.T) {
	base := makeBase()

	aw, err := AssembleFromExtension(base, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(aw.Tasks) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(aw.Tasks))
	}
	if aw.Tasks[0].Number != "0000" {
		t.Errorf("first task number = %q, want \"0000\"", aw.Tasks[0].Number)
	}
	if aw.Tasks[1].Number != "FINAL" {
		t.Errorf("last task number = %q, want \"FINAL\"", aw.Tasks[1].Number)
	}
}

// TestAssembleFromExtension_TokenMerge verifies that base tokens are overridden
// by extension tokens.
func TestAssembleFromExtension_TokenMerge(t *testing.T) {
	base := makeBase()
	ext := &WorkflowExtension{
		Tokens: map[string]string{"Deck": "override", "Extra": "val"},
		Phases: []Phase{{Title: "One", Jobs: []string{"@j"}}},
	}
	tokens := map[string]string{"Deck": "original", "Branch": "main"}

	aw, err := AssembleFromExtension(base, ext, tokens)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if aw.Tokens["Deck"] != "override" {
		t.Errorf("Deck token = %q, want \"override\"", aw.Tokens["Deck"])
	}
	if aw.Tokens["Branch"] != "main" {
		t.Errorf("Branch token = %q, want \"main\"", aw.Tokens["Branch"])
	}
	if aw.Tokens["Extra"] != "val" {
		t.Errorf("Extra token = %q, want \"val\"", aw.Tokens["Extra"])
	}
}

// TestAssembleFromPlanExtension verifies plan assembly: base fragments first,
// then extension fragments.
func TestAssembleFromPlanExtension(t *testing.T) {
	base := BuiltinBase{
		Type:      "plan",
		Fragments: []string{"@base-frag"},
	}
	ext := &PlanExtension{
		Fragments: []string{"@ext-frag"},
	}

	aw, err := AssembleFromPlanExtension(base, ext, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(aw.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(aw.Tasks))
	}
	want := []string{"@base-frag", "@ext-frag"}
	got := aw.Tasks[0].Fragments
	if len(got) != len(want) {
		t.Fatalf("fragments = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("fragments[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
