package provision

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// DefaultGitignoreEntries is used when setup.yaml does not specify gitignore.entries.
var DefaultGitignoreEntries = []string{
	".claude/",
	"*.md",
	"!README.md",
}

// EnsureGitignore writes any entries from the list that are not already present
// in <dir>/.gitignore. Existing content is never modified — new entries are
// appended under an hv-managed header. Idempotent.
func EnsureGitignore(dir string, entries []string) error {
	if len(entries) == 0 {
		return nil
	}
	path := filepath.Join(dir, ".gitignore")

	existing := make(map[string]bool)
	if f, err := os.Open(path); err == nil {
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" && !strings.HasPrefix(line, "#") {
				existing[line] = true
			}
		}
		_ = f.Close()
	}

	var missing []string
	for _, e := range entries {
		trimmed := strings.TrimSpace(e)
		if trimmed != "" && !existing[trimmed] {
			missing = append(missing, e)
		}
	}
	if len(missing) == 0 {
		return nil
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "\n# hv-managed (added by hv deck provision — idempotent)\n"); err != nil {
		return err
	}
	for _, e := range missing {
		if _, err := fmt.Fprintln(f, e); err != nil {
			return err
		}
	}
	return nil
}

// EnsureReadme creates a minimal README.md at dir if one does not already exist.
// Existing README.md files are never overwritten.
func EnsureReadme(dir, heading string) error {
	path := filepath.Join(dir, "README.md")
	if _, err := os.Stat(path); err == nil {
		return nil
	}
	content := fmt.Sprintf("# %s\n", heading)
	return os.WriteFile(path, []byte(content), 0o644)
}
