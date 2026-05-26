// Package resolve turns a validated workspace config into the concrete plan that
// provision/teardown execute: every repo's clone URL, branch, and destination
// path, plus workspace and launch paths the .code-workspace generator needs.
package resolve

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/talkersoft/hive-deck/internal/config"
)

// RepoPlan describes a single repo to be cloned (or already cloned).
type RepoPlan struct {
	Deck       string
	NodePath   string // relative path from workspace root, e.g. "vm-infra/cloud-manager"
	Module     string // module name if from modules:, else ""
	Repo       string
	URL        string // resolved clone URL
	Branch     string // resolved branch
	Dest       string // absolute destination path on disk
	GitHubSlug string // "<owner>/<repo>" if the org is on github.com, else ""
}

// SymlinkPlan describes a single symlink to be created during provision.
type SymlinkPlan struct {
	Dest   string // absolute path of the symlink itself
	Target string // expanded target path
	Name   string // symlink name (basename of target, leading dot stripped)
}

// Plan describes everything a workspace run needs.
type Plan struct {
	Deck             string
	DeckDir   string   // <workspaces_root>/<workspace>
	LaunchDir        string   // <workspaces_root>/launch  (shared across all workspaces)
	LaunchFile       string   // <workspaces_root>/launch/<workspace>.code-workspace
	Folders          []string // all tree node directories to create and scaffold, in tree order
	Repos            []RepoPlan
	Symlinks         []SymlinkPlan
	WorkspaceFolders []string // node dirs with workspace_folder: true
}

// Build expands the full workspace tree into a flat Plan.
// The caller must have already called Loaded.ValidateDeck.
func Build(l *config.Loaded) (*Plan, error) {
	root, err := config.ExpandRoot(l.Setup.Deck.Root)
	if err != nil {
		return nil, err
	}
	wsDir := filepath.Join(root, l.DeckName)
	launchDir := filepath.Join(root, config.LaunchDir)
	launchFile := filepath.Join(launchDir, l.DeckName+".code-workspace")

	plan := &Plan{
		Deck: l.DeckName,
		DeckDir: wsDir,
		LaunchDir:    launchDir,
		LaunchFile:   launchFile,
		Folders:      []string{wsDir},
	}

	if err := walkNode(l, wsDir, "", l.DeckFile.Deck, plan); err != nil {
		return nil, err
	}
	return plan, nil
}

// AllRepos returns every RepoPlan for the workspace.
// Used by workspace.Regenerate to write the .code-workspace file.
func AllRepos(l *config.Loaded) ([]RepoPlan, error) {
	plan, err := Build(l)
	if err != nil {
		return nil, err
	}
	return plan.Repos, nil
}

// AllWorkspaceFolders returns every WorkspaceFolder path for the full workspace tree.
// Used by workspace.Regenerate to include non-repo folders in the .code-workspace file.
func AllWorkspaceFolders(l *config.Loaded) ([]string, error) {
	plan, err := Build(l)
	if err != nil {
		return nil, err
	}
	return plan.WorkspaceFolders, nil
}

// walkNode recursively walks a TreeNode, collecting folders, repos, symlinks, and
// workspace_folder entries. nodeDir is the absolute path of the current node.
// nodePath is the relative display path from the workspace root.
func walkNode(l *config.Loaded, nodeDir, nodePath string, node config.TreeNode, plan *Plan) error {
	if err := collectRepoRefs(l, nodeDir, nodePath, node.RepoRefs, plan); err != nil {
		return err
	}
	if err := collectModuleRefs(l, nodeDir, nodePath, node.ModuleRefs, plan); err != nil {
		return err
	}
	if err := collectSymlinks(nodeDir, node.Symlinks, plan); err != nil {
		return err
	}
	if node.WorkspaceFolder {
		plan.WorkspaceFolders = append(plan.WorkspaceFolders, nodeDir)
	}

	childNames := sortedKeys(node.Children)
	for _, childName := range childNames {
		child := node.Children[childName]
		childDir := filepath.Join(nodeDir, childName)
		childPath := childName
		if nodePath != "" {
			childPath = nodePath + "/" + childName
		}
		plan.Folders = append(plan.Folders, childDir)
		if err := walkNode(l, childDir, childPath, child, plan); err != nil {
			return err
		}
	}
	return nil
}

// collectRepoRefs adds RepoPlans for all repos: entries at a given tree node.
// Each entry is "org/repo"; repos land flat at nodeDir/repo.
func collectRepoRefs(l *config.Loaded, nodeDir, nodePath string, repoRefs []string, plan *Plan) error {
	for _, ref := range repoRefs {
		parts := strings.SplitN(ref, "/", 2)
		orgName, repoName := parts[0], parts[1]
		org := l.Setup.Orgs[orgName]
		url, err := cloneURL(org, repoName)
		if err != nil {
			return err
		}
		plan.Repos = append(plan.Repos, RepoPlan{
			Deck: l.DeckName,
			NodePath:   nodePath,
			Module:     "",
			Repo:       repoName,
			URL:        url,
			Branch:     branchFor(repoName, l.Setup),
			Dest:       filepath.Join(nodeDir, repoName),
			GitHubSlug: githubSlug(org, repoName),
		})
	}
	return nil
}

// collectModuleRefs adds RepoPlans for all modules: entries at a given tree node.
// Module repos are looked up in l.Modules and land flat at nodeDir/repo.
func collectModuleRefs(l *config.Loaded, nodeDir, nodePath string, moduleRefs []string, plan *Plan) error {
	for _, modName := range moduleRefs {
		mod := l.Modules[modName]
		org := l.Setup.Orgs[mod.Org]
		for _, repo := range mod.Repos {
			url, err := cloneURL(org, repo)
			if err != nil {
				return err
			}
			plan.Repos = append(plan.Repos, RepoPlan{
				Deck: l.DeckName,
				NodePath:   nodePath,
				Module:     modName,
				Repo:       repo,
				URL:        url,
				Branch:     branchFor(repo, l.Setup),
				Dest:       filepath.Join(nodeDir, repo),
				GitHubSlug: githubSlug(org, repo),
			})
		}
	}
	return nil
}

// collectSymlinks records a SymlinkPlan for each target in the symlinks: list.
// Symlinks are created inside nodeDir; the name is derived from the target basename
// with any leading dot stripped (e.g. ~/.hive → "hive").
func collectSymlinks(nodeDir string, targets []string, plan *Plan) error {
	for _, target := range targets {
		expanded, err := expandHome(target)
		if err != nil {
			return err
		}
		name := strings.TrimPrefix(filepath.Base(expanded), ".")
		if name == "" {
			name = filepath.Base(expanded)
		}
		plan.Symlinks = append(plan.Symlinks, SymlinkPlan{
			Dest:   filepath.Join(nodeDir, name),
			Target: expanded,
			Name:   name,
		})
	}
	return nil
}

func expandHome(path string) (string, error) {
	if path == "~" || strings.HasPrefix(path, "~/") {
		h, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if path == "~" {
			return h, nil
		}
		return filepath.Join(h, path[2:]), nil
	}
	return path, nil
}

func cloneURL(org config.Org, repo string) (string, error) {
	host, orgPath, ok := splitOrgURL(org.URL)
	if !ok {
		return "", fmt.Errorf("org url %q must be of the form <host>/<org>", org.URL)
	}
	switch org.Protocol {
	case "ssh":
		return fmt.Sprintf("git@%s:%s/%s.git", host, orgPath, repo), nil
	case "https":
		return fmt.Sprintf("https://%s/%s/%s.git", host, orgPath, repo), nil
	default:
		return "", fmt.Errorf("unknown protocol %q", org.Protocol)
	}
}

// githubSlug returns "<owner>/<repo>" when the org is hosted on github.com, else "".
func githubSlug(org config.Org, repo string) string {
	host, orgPath, ok := splitOrgURL(org.URL)
	if !ok || host != "github.com" {
		return ""
	}
	return orgPath + "/" + repo
}

func splitOrgURL(s string) (host, org string, ok bool) {
	s = strings.TrimSuffix(s, "/")
	i := strings.Index(s, "/")
	if i < 0 {
		return "", "", false
	}
	host = s[:i]
	org = s[i+1:]
	if host == "" || org == "" {
		return "", "", false
	}
	return host, org, true
}

func branchFor(repo string, s config.Setup) string {
	if b, ok := s.Branches[repo]; ok && b != "" {
		return b
	}
	return s.DefaultBranch
}

func sortedKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
