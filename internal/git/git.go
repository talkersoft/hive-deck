// Package git wraps the small set of git operations hv needs.
// Auth is assumed preconfigured (SSH agent, credential helper, etc.).
package git

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Pull performs `git pull` in the repo at dir, streaming output to the caller.
func Pull(dir string) error {
	cmd := exec.Command("git", "pull")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git pull in %s: %w", dir, err)
	}
	return nil
}

// Clone performs `git clone --branch <branch> <url> <dest>`.
// Streams git's output to the caller's stdout/stderr.
func Clone(url, branch, dest string) error {
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	cmd := exec.Command("git", "clone", "--branch", branch, url, dest)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git clone %s: %w", url, err)
	}
	return nil
}

// CleanStatus describes whether a repo is in a safe-to-delete state.
type CleanStatus struct {
	Repo    string
	Clean   bool
	Reasons []string
}

// Check inspects the repo at dir and reports every reason it's not safe to
// remove. Reasons are accumulated, not short-circuited. Untracked files count
// as "not clean".
func Check(dir, repo string) CleanStatus { return checkRepo(dir, repo, false) }

// CheckTracked is like Check but treats untracked files as harmless. Use it
// when the caller plans to preserve untracked files separately (surgical
// teardown).
func CheckTracked(dir, repo string) CleanStatus { return checkRepo(dir, repo, true) }

func checkRepo(dir, repo string, ignoreUntracked bool) CleanStatus {
	st := CleanStatus{Repo: repo}

	if !isGitRepo(dir) {
		st.Reasons = append(st.Reasons, "not a git repository")
		return st
	}

	args := []string{"status", "--porcelain"}
	dirtyMsg := "working tree not clean (uncommitted or untracked changes)"
	if ignoreUntracked {
		args = append(args, "--untracked-files=no")
		dirtyMsg = "uncommitted changes to tracked files"
	}
	if out, err := run(dir, "git", args...); err != nil {
		st.Reasons = append(st.Reasons, "git status failed: "+err.Error())
	} else if strings.TrimSpace(out) != "" {
		st.Reasons = append(st.Reasons, dirtyMsg)
	}

	branch, err := run(dir, "git", "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		st.Reasons = append(st.Reasons, "could not read HEAD: "+err.Error())
	} else if strings.TrimSpace(branch) == "HEAD" {
		st.Reasons = append(st.Reasons, "detached HEAD")
	}

	if _, err := run(dir, "git", "rev-parse", "--abbrev-ref", "--symbolic-full-name", "@{upstream}"); err != nil {
		st.Reasons = append(st.Reasons, "no upstream branch configured (cannot verify pushed state)")
	} else {
		ahead, err := run(dir, "git", "rev-list", "--count", "@{upstream}..HEAD")
		if err != nil {
			st.Reasons = append(st.Reasons, "could not compute ahead count: "+err.Error())
		} else if strings.TrimSpace(ahead) != "0" {
			st.Reasons = append(st.Reasons, "local commits not pushed to upstream")
		}
	}

	if out, err := run(dir, "git", "stash", "list"); err != nil {
		st.Reasons = append(st.Reasons, "git stash list failed: "+err.Error())
	} else if strings.TrimSpace(out) != "" {
		st.Reasons = append(st.Reasons, "stash entries present")
	}

	st.Clean = len(st.Reasons) == 0
	return st
}

// RestoreInPlace recovers a torn-down "shell" repo dir (no .git/, no tracked
// source — only the untracked files surgical teardown preserved) back into a
// functional git repo on `branch`. It clones url@branch into a temp dir,
// then copies anything missing from dest (including .git/) using
// `rsync -a --ignore-existing`, so preserved untracked files in dest are
// never overwritten.
//
// Path conflicts where a preserved file shadows a tracked file are silent —
// rsync just skips writing the tracked version. After the restore, git
// status will show those paths as locally modified (their preserved content
// differs from origin), which is the truthful state.
func RestoreInPlace(dir, url, branch string) error {
	tmp, err := os.MkdirTemp("", "hv-restore-")
	if err != nil {
		return fmt.Errorf("mktemp: %w", err)
	}
	defer os.RemoveAll(tmp)

	target := filepath.Join(tmp, "clone")
	if err := Clone(url, branch, target); err != nil {
		return err
	}

	cmd := exec.Command("rsync", "-a", "--ignore-existing", target+"/", dir+"/")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rsync restore into %s: %w", dir, err)
	}
	return nil
}

// ListTrackedFiles returns the paths of every file currently tracked by git
// in the repo at dir, relative to dir. Uses NUL-separated output so paths
// with unusual characters round-trip safely.
func ListTrackedFiles(dir string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return nil, fmt.Errorf("git ls-files: %s", strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, err
	}
	parts := bytes.Split(out, []byte{0})
	files := make([]string, 0, len(parts))
	for _, p := range parts {
		if len(p) == 0 {
			continue
		}
		files = append(files, string(p))
	}
	return files, nil
}

func isGitRepo(dir string) bool {
	fi, err := os.Stat(filepath.Join(dir, ".git"))
	if err != nil {
		return false
	}
	return fi.IsDir() || !fi.IsDir() // either dir (normal) or file (worktree/submodule)
}

func run(dir, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return string(out), fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return string(out), err
	}
	return string(out), nil
}
