// Package workspace generates the single VS Code .code-workspace file that
// every hv deck run produces.
//
// Layout:
//
//	<workspaces_root>/launch/<workspace>.code-workspace
//
// The file's folders[] contains one entry per repo on disk, with a final "."
// entry pointing at the launch folder itself. The "." entry must stay last
// (idempotency contract).
//
// Regenerate is called once at the end of every provision/teardown/prune run.
// When zero repos remain on disk, the file is removed.
// The launch directory is created on demand and never removed by teardown.
package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/resolve"
)

type folder struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

type file struct {
	Folders []folder `json:"folders"`
}

// Regenerate writes (or removes) the workspace-level .code-workspace file at
// <workspaces_root>/launch/<workspace>.code-workspace. It re-derives all repos
// from the full workspace tree (not just the in-scope filter) so the file
// always reflects the complete on-disk state.
func Regenerate(plan *resolve.Plan, l *config.Loaded) error {
	allRepos, err := resolve.AllRepos(l)
	if err != nil {
		return err
	}
	var onDisk []resolve.RepoPlan
	for _, r := range allRepos {
		if fi, err := os.Stat(r.Dest); err == nil && fi.IsDir() {
			onDisk = append(onDisk, r)
		}
	}

	if len(onDisk) == 0 {
		return removeIfExists(plan.LaunchFile)
	}

	if err := os.MkdirAll(plan.LaunchDir, 0o755); err != nil {
		return err
	}

	f := file{Folders: make([]folder, 0, len(onDisk)+1)}
	for _, r := range onDisk {
		rel, err := filepath.Rel(plan.LaunchDir, r.Dest)
		if err != nil {
			return err
		}
		f.Folders = append(f.Folders, folder{Name: r.Repo, Path: rel})
	}

	// Launch folder itself — sidebar entry listing .code-workspace files.
	// MUST stay last in folders[] (idempotency contract).
	f.Folders = append(f.Folders, folder{Name: config.LaunchDir, Path: "."})

	b, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	b = append(b, '\n')
	return os.WriteFile(plan.LaunchFile, b, 0o644)
}

func removeIfExists(path string) error {
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
