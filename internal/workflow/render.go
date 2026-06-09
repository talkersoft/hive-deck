package workflow

import (
	"bytes"
	"fmt"
	"io/fs"
	"strings"
	"text/template"

	"github.com/talkersoft/hive-deck/internal/config"
)

// TemplateData is the context passed to every fragment template.
// Fields map to the existing token names so fragments using {{.Deck}} etc.
// continue to work. Config-derived fields enable conditional logic.
type TemplateData struct {
	// Workflow tokens
	Deck       string
	Branch     string
	DeckRoot   string
	ExecFolder string
	PlanFolder string
	// Ship config (from deck config.yaml)
	PRMode              string
	OpenBrowser         bool
	DeleteBranchOnMerge bool
	// Agent type (from deck yaml agent: field) — e.g. "claude" or "codex"
	Agent string
}

// TokensToData converts a legacy map[string]string into a TemplateData.
// Config-derived fields default to zero values when not supplied.
func TokensToData(tokens map[string]string) TemplateData {
	return TemplateData{
		Deck:       tokens["Deck"],
		Branch:     tokens["Branch"],
		DeckRoot:   tokens["DeckRoot"],
		ExecFolder: tokens["ExecFolder"],
		PlanFolder: tokens["PlanFolder"],
	}
}

// Render loads each fragment ref, executes it as a Go template against data,
// and concatenates the results separated by newlines.
// deck is used for the deck-scoped fragment lookup (step 1); pass "" to skip.
func Render(root, deck string, fragments []string, data TemplateData, fallbackFS fs.FS) (string, error) {
	var parts []string
	for _, ref := range fragments {
		content, err := config.LoadFragmentWithFallback(root, ref, deck, fallbackFS)
		if err != nil {
			return "", fmt.Errorf("render: %w", err)
		}
		tmpl, err := template.New(ref).Option("missingkey=error").Parse(content)
		if err != nil {
			return "", fmt.Errorf("render: fragment %q: %w", ref, err)
		}
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			// Provide a friendlier message for the common migration mistake.
			if strings.Contains(err.Error(), "function") {
				return "", fmt.Errorf("render: fragment %q uses bare {{Key}} syntax — migrate to {{.Key}}: %w", ref, err)
			}
			return "", fmt.Errorf("render: fragment %q: %w", ref, err)
		}
		parts = append(parts, buf.String())
	}
	return strings.Join(parts, "\n"), nil
}
