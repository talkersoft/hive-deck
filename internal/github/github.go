// Package github wraps the small set of GitHub operations hv needs.
// All operations shell out to the `gh` CLI; auth is assumed preconfigured.
package github

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

// EnsureAvailable returns an error if the gh CLI is not on PATH.
// Call this once at the start of any run that may need github operations.
func EnsureAvailable() error {
	if _, err := exec.LookPath("gh"); err != nil {
		return errors.New("gh CLI is required for --create-missing; install from https://cli.github.com and run `gh auth login`")
	}
	return nil
}

// Exists reports whether owner/repo exists on github.com.
// Returns (true, nil) if found, (false, nil) if confirmed-not-found, and
// (false, err) for any other failure (auth, network, gh missing) so callers
// don't accidentally treat an outage as "doesn't exist, create it."
func Exists(slug string) (bool, error) {
	cmd := exec.Command("gh", "repo", "view", slug, "--json", "name")
	out, err := cmd.CombinedOutput()
	if err == nil {
		return true, nil
	}
	text := strings.TrimSpace(string(out))
	if isNotFound(text) {
		return false, nil
	}
	return false, fmt.Errorf("gh repo view %s: %s", slug, text)
}

// Create makes a private github.com repository with an initial README.
// Equivalent to `gh repo create <slug> --private --add-readme`.
func Create(slug string) error {
	cmd := exec.Command("gh", "repo", "create", slug, "--private", "--add-readme")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("gh repo create %s: %s", slug, strings.TrimSpace(string(out)))
	}
	return nil
}

// isNotFound recognises the various ways the gh CLI signals a missing repo.
// We only want to treat *confirmed* not-found as "safe to create" — auth or
// permission errors must surface as real errors, not silent creation attempts.
func isNotFound(text string) bool {
	switch {
	case strings.Contains(text, "Could not resolve to a Repository"):
		return true
	case strings.Contains(text, "GraphQL: Could not resolve"):
		return true
	case strings.Contains(text, "HTTP 404"):
		return true
	}
	return false
}
