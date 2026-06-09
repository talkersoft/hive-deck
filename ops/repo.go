package ops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/deckfile"
	"github.com/talkersoft/hive-deck/internal/git"
	gh "github.com/talkersoft/hive-deck/internal/github"
	"github.com/talkersoft/hive-deck/internal/provision"
	"github.com/talkersoft/hive-deck/internal/resolve"
	"github.com/talkersoft/hive-deck/internal/workspace"
)

func RunRepoList(in RepoListInput) (string, error) {
	l, err := config.LoadDeck(in.Deck)
	if err != nil {
		return "", err
	}
	switch in.Target {
	case "repos":
		return listRepos(l)
	case "modules":
		return listModules(l)
	case "nodes":
		return listNodes(l)
	default:
		return "", fmt.Errorf("target must be one of: repos, modules, nodes")
	}
}

func listRepos(l *config.Loaded) (string, error) {
	root, err := config.ExpandRoot(l.Setup.Workspace.Root)
	if err != nil {
		return "", err
	}
	wsDir := filepath.Join(root, l.DeckName)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-45s %-30s %s\n", "REPO", "NODE", "ON DISK"))
	walkRepoList(&sb, l.DeckFile.Deck, wsDir, "", l, root)
	return sb.String(), nil
}

func walkRepoList(sb *strings.Builder, node config.TreeNode, nodeDir, nodePath string, l *config.Loaded, wsRoot string) {
	wsDir := filepath.Join(wsRoot, l.DeckName)
	for _, ref := range node.RepoRefs {
		parts := strings.SplitN(ref, "/", 2)
		repoName := parts[1]
		dest := filepath.Join(nodeDir, repoName)
		onDisk := "no"
		if fi, err := os.Stat(dest); err == nil && fi.IsDir() {
			onDisk = "yes"
		}
		rel := strings.TrimPrefix(dest, wsDir+"/")
		nodeLabel := nodePath
		if nodeLabel == "" {
			nodeLabel = "(root)"
		}
		sb.WriteString(fmt.Sprintf("%-45s %-30s %s\n", ref, nodeLabel, onDisk))
		_ = rel
	}
	for _, modName := range node.ModuleRefs {
		mod := l.Modules[modName]
		for _, repo := range mod.Repos {
			dest := filepath.Join(nodeDir, repo)
			onDisk := "no"
			if fi, err := os.Stat(dest); err == nil && fi.IsDir() {
				onDisk = "yes"
			}
			nodeLabel := nodePath
			if nodeLabel == "" {
				nodeLabel = "(root)"
			}
			ref := fmt.Sprintf("%s/%s [module:%s]", mod.Org, repo, modName)
			sb.WriteString(fmt.Sprintf("%-45s %-30s %s\n", ref, nodeLabel, onDisk))
			_ = dest
		}
	}
	for childName, child := range node.Children {
		childPath := childName
		if nodePath != "" {
			childPath = nodePath + "/" + childName
		}
		walkRepoList(sb, child, filepath.Join(nodeDir, childName), childPath, l, wsRoot)
	}
}

func listModules(l *config.Loaded) (string, error) {
	if len(l.Modules) == 0 {
		return "no modules defined in modules.yaml\n", nil
	}
	var sb strings.Builder
	for name, mod := range l.Modules {
		sb.WriteString(fmt.Sprintf("module: %s (org: %s)\n", name, mod.Org))
		for _, repo := range mod.Repos {
			sb.WriteString(fmt.Sprintf("  %s/%s\n", mod.Org, repo))
		}
	}
	return sb.String(), nil
}

func listNodes(l *config.Loaded) (string, error) {
	var sb strings.Builder
	sb.WriteString("(root)\n")
	collectNodes(&sb, l.DeckFile.Deck.Children, "")
	return sb.String(), nil
}

func collectNodes(sb *strings.Builder, children map[string]config.TreeNode, prefix string) {
	for name, child := range children {
		path := name
		if prefix != "" {
			path = prefix + "/" + name
		}
		sb.WriteString(path + "\n")
		collectNodes(sb, child.Children, path)
	}
}

func RunRepoAdd(in RepoAddInput) (string, error) {
	l, err := config.LoadDeck(in.Deck)
	if err != nil {
		return "", err
	}

	// Validate repo ref
	parts := strings.SplitN(in.Repo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", fmt.Errorf("repo must be org/repo (e.g. talkersoft-com/my-service)")
	}
	orgName, repoName := parts[0], parts[1]
	org, ok := l.Setup.Orgs[orgName]
	if !ok {
		return "", fmt.Errorf("org %q not defined in config.yaml", orgName)
	}

	// Resolve dest
	root, err := config.ExpandRoot(l.Setup.Workspace.Root)
	if err != nil {
		return "", err
	}
	nodeDir := filepath.Join(root, l.DeckName)
	if in.Node != "" {
		for _, part := range strings.Split(in.Node, "/") {
			nodeDir = filepath.Join(nodeDir, part)
		}
	}
	dest := filepath.Join(nodeDir, repoName)

	// Resolve branch
	branch := in.Branch
	if branch == "" {
		branch = l.DeckFile.Branch
	}
	if branch == "" {
		branch = "main"
	}

	// Resolve commit message
	message := in.Message
	if message == "" {
		message = "initial commit"
	}

	// Build clone URL and slug
	cloneURL, err := buildRepoCloneURL(org, repoName)
	if err != nil {
		return "", err
	}
	slug := buildRepoGitHubSlug(org, repoName)

	// Ensure GitHub repo exists
	exists, err := gh.Exists(slug)
	if err != nil {
		return "", fmt.Errorf("check github repo: %w", err)
	}
	if !exists {
		if err := gh.Create(slug); err != nil {
			return "", fmt.Errorf("create github repo: %w", err)
		}
	}

	// Handle disk state
	action, err := ensureRepoDisk(dest, cloneURL, branch, message)
	if err != nil {
		return "", err
	}

	// Edit deck YAML
	if err := deckfile.AddRepoToDeckFile(l.DeckPath, in.Node, in.Repo); err != nil {
		return "", fmt.Errorf("update deck YAML: %w", err)
	}

	// Optionally add to module
	if in.Module != "" {
		modulesPath, err := config.FindModulesPath()
		if err != nil {
			return "", fmt.Errorf("find modules.yaml: %w", err)
		}
		if modulesPath == "" {
			return "", fmt.Errorf("modules.yaml not found — create it first")
		}
		if err := deckfile.AddRepoToModule(modulesPath, in.Module, orgName, repoName); err != nil {
			return "", fmt.Errorf("update modules.yaml: %w", err)
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("repo_add: %s\n", in.Repo))
	sb.WriteString(fmt.Sprintf("  action:  %s\n", action))
	sb.WriteString(fmt.Sprintf("  dest:    %s\n", dest))
	sb.WriteString(fmt.Sprintf("  remote:  github.com/%s\n", slug))
	sb.WriteString(fmt.Sprintf("  branch:  %s\n", branch))
	sb.WriteString(fmt.Sprintf("  yaml:    updated %s.yaml → %s\n", l.DeckName, nodeOrRoot(in.Node)))
	if in.Module != "" {
		sb.WriteString(fmt.Sprintf("  module:  added to %s in modules.yaml\n", in.Module))
	}

	// Reload config (YAML was just edited) and run per-repo init steps:
	// gitignore scaffold for the repo, then workspace file regeneration.
	if l2, err2 := config.LoadDeck(in.Deck); err2 == nil {
		ruleset := nodeGitignoreRuleset(l2.DeckFile.Deck, in.Node)
		if err2 = provision.ApplyRepoScaffold(dest, ruleset, l2.Setup); err2 != nil {
			sb.WriteString(fmt.Sprintf("  warn: gitignore scaffold: %v\n", err2))
		}
		if plan, err2 := resolve.Build(l2); err2 == nil {
			if err2 = workspace.Regenerate(plan, l2); err2 != nil {
				sb.WriteString(fmt.Sprintf("  warn: workspace regenerate: %v\n", err2))
			} else {
				sb.WriteString("  workspace: regenerated .code-workspace\n")
			}
		}
	}

	return sb.String(), nil
}

// nodeGitignoreRuleset walks the deck tree to find the gitignore_ruleset
// configured on the node at nodePath (slash-separated). Returns "" if not set.
func nodeGitignoreRuleset(root config.TreeNode, nodePath string) string {
	if nodePath == "" {
		return root.GitignoreRuleset
	}
	cur := root
	for _, part := range strings.Split(nodePath, "/") {
		child, ok := cur.Children[part]
		if !ok {
			return ""
		}
		cur = child
	}
	return cur.GitignoreRuleset
}

func ensureRepoDisk(dest, cloneURL, branch, message string) (string, error) {
	fi, statErr := os.Stat(dest)
	if os.IsNotExist(statErr) {
		// Clone fresh
		if err := git.Clone(cloneURL, branch, dest); err != nil {
			// If branch doesn't exist on remote, clone default and create branch
			if err2 := gitCloneAndBranch(cloneURL, branch, dest); err2 != nil {
				return "", fmt.Errorf("clone: %w", err)
			}
		}
		return "cloned fresh", nil
	}
	if statErr != nil {
		return "", fmt.Errorf("stat %s: %w", dest, statErr)
	}
	if !fi.IsDir() {
		return "", fmt.Errorf("%s exists but is not a directory", dest)
	}

	// Check for .git
	if _, err := os.Stat(filepath.Join(dest, ".git")); err == nil {
		// Existing git repo — set remote, push
		if err := gitSetRemote(dest, cloneURL); err != nil {
			return "", err
		}
		if err := gitPush(dest, branch); err != nil {
			return "", err
		}
		return "pushed existing repo", nil
	}

	// Folder exists, no .git — init and push
	if err := gitInitAndPush(dest, cloneURL, branch, message); err != nil {
		return "", err
	}
	return "initialized and pushed existing folder", nil
}

func gitCloneAndBranch(cloneURL, branch, dest string) error {
	// Clone without specifying branch (use remote default), then create branch
	cmd := exec.Command("git", "clone", cloneURL, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git clone: %s", strings.TrimSpace(string(out)))
	}
	cmd = exec.Command("git", "checkout", "-b", branch)
	cmd.Dir = dest
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git checkout -b %s: %s", branch, strings.TrimSpace(string(out)))
	}
	return nil
}

func gitSetRemote(dir, url string) error {
	// Check if origin exists
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	if out, err := cmd.Output(); err == nil {
		if strings.TrimSpace(string(out)) == url {
			return nil // already correct
		}
		// Update it
		cmd = exec.Command("git", "remote", "set-url", "origin", url)
		cmd.Dir = dir
	} else {
		// Add it
		cmd = exec.Command("git", "remote", "add", "origin", url)
		cmd.Dir = dir
	}
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git remote: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func gitPush(dir, branch string) error {
	cmd := exec.Command("git", "push", "-u", "origin", "HEAD:"+branch)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git push: %s", strings.TrimSpace(string(out)))
	}
	return nil
}

func gitInitAndPush(dir, url, branch, message string) error {
	run := func(args ...string) error {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
		}
		return nil
	}
	if err := run("git", "init", "-b", branch); err != nil {
		// Older git: init then rename branch
		if err2 := run("git", "init"); err2 != nil {
			return err2
		}
		_ = run("git", "checkout", "-b", branch) // best-effort
	}
	if err := run("git", "remote", "add", "origin", url); err != nil {
		return err
	}
	// Stage all files (if any)
	_ = run("git", "add", "-A") // ignore error if empty
	// Commit only if there's something staged
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = dir
	out, _ := cmd.Output()
	if len(strings.TrimSpace(string(out))) > 0 {
		if err := run("git", "commit", "-m", message); err != nil {
			return err
		}
	}
	// Force push: GitHub auto-creates a README commit on the remote when the
	// repo is created, so a regular push would be rejected with "fetch first".
	// Local content is canonical in the retro-provision case.
	if err := run("git", "push", "--force", "-u", "origin", branch); err != nil {
		if err2 := run("git", "push", "--force", "-u", "origin", "HEAD"); err2 != nil {
			return err
		}
	}
	return nil
}

func buildRepoCloneURL(org config.Org, repo string) (string, error) {
	host, orgPath, ok := splitOrgURLForRepo(org.URL)
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

func buildRepoGitHubSlug(org config.Org, repo string) string {
	host, orgPath, ok := splitOrgURLForRepo(org.URL)
	if !ok || host != "github.com" {
		return ""
	}
	return orgPath + "/" + repo
}

func splitOrgURLForRepo(s string) (host, orgPath string, ok bool) {
	s = strings.TrimSuffix(s, "/")
	i := strings.Index(s, "/")
	if i < 0 {
		return "", "", false
	}
	return s[:i], s[i+1:], s[:i] != "" && s[i+1:] != ""
}

func nodeOrRoot(node string) string {
	if node == "" {
		return "deck root"
	}
	return node
}
