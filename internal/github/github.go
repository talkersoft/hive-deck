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

// PRInfo describes the pull-request state for a branch.
type PRInfo struct {
	State string // "OPEN", "MERGED", "CLOSED", or "" when no PR exists
	URL   string
}

// GetPRInfo returns the state and URL of the most recent PR for the given
// branch. Returns a zero PRInfo (State == "") when no PR is found.
func GetPRInfo(dir, branch string) PRInfo {
	cmd := exec.Command("gh", "pr", "view", branch, "--json", "state,url", "--jq", ".state+\"\\t\"+.url")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return PRInfo{}
	}
	parts := strings.SplitN(strings.TrimSpace(string(out)), "\t", 2)
	if len(parts) != 2 || parts[0] == "" {
		return PRInfo{}
	}
	return PRInfo{State: parts[0], URL: parts[1]}
}

// OpenPR describes an open pull request returned by ListOpenPRs.
type OpenPR struct {
	Number int
	Title  string
	URL    string
	Branch string
}

// ListOpenPRs returns all open pull requests for the repo at dir.
// Returns nil (not an error) when the repo has no open PRs.
func ListOpenPRs(dir string) []OpenPR {
	cmd := exec.Command("gh", "pr", "list", "--state", "open", "--json", "number,title,url,headRefName", "--jq", ".[] | [.number|tostring, .headRefName, .url, .title] | join(\"\t\")")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var prs []OpenPR
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		if line == "" {
			continue
		}
		parts := strings.SplitN(line, "\t", 4)
		if len(parts) != 4 {
			continue
		}
		var n int
		fmt.Sscanf(parts[0], "%d", &n)
		prs = append(prs, OpenPR{Number: n, Branch: parts[1], URL: parts[2], Title: parts[3]})
	}
	return prs
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
