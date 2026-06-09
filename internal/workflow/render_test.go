package workflow

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenderDotSyntax(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "fragments"), 0755)
	os.WriteFile(filepath.Join(root, "fragments", "greet.md"), []byte("deck={{.Deck}} mode={{.PRMode}}\n"), 0644)

	data := TemplateData{Deck: "my-deck", PRMode: "await_merge"}
	got, err := Render(root, []string{"@greet"}, data, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := "deck=my-deck mode=await_merge\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestRenderConditional(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "fragments"), 0755)
	os.WriteFile(filepath.Join(root, "fragments", "cond.md"), []byte(`{{if eq .PRMode "await_merge"}}polling{{else}}manual{{end}}`), 0644)

	for _, tc := range []struct{ mode, want string }{
		{"await_merge", "polling"},
		{"manual", "manual"},
	} {
		data := TemplateData{PRMode: tc.mode}
		got, err := Render(root, []string{"@cond"}, data, nil)
		if err != nil {
			t.Fatalf("mode=%s unexpected error: %v", tc.mode, err)
		}
		if got != tc.want {
			t.Errorf("mode=%s: got %q, want %q", tc.mode, got, tc.want)
		}
	}
}
