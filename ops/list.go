package ops

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/github"
	"github.com/talkersoft/hive-deck/internal/resolve"
)

func RunListRepos(in ListReposInput) (string, error) {
	l, err := config.LoadDeck(in.Deck)
	if err != nil {
		return "", err
	}
	root, err := config.ExpandRoot(l.Setup.Workspace.Root)
	if err != nil {
		return "", err
	}
	wsDir := filepath.Join(root, l.DeckName)
	if err := l.ValidateDeck(); err != nil {
		return "", err
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%-40s %-25s %s\n", "DEST", "MODULE", "PROVISIONED"))
	if err := walkListNodeToString(&sb, l.DeckFile.Deck, wsDir, "", l); err != nil {
		return "", err
	}
	return sb.String(), nil
}

func RunListPulls(in ListPullsInput) (string, error) {
	l, err := config.LoadDeck(in.Deck)
	if err != nil {
		return "", err
	}
	if err := l.ValidateDeck(); err != nil {
		return "", err
	}
	plan, err := resolve.Build(l)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	total := 0
	for _, repo := range plan.Repos {
		if fi, serr := os.Stat(repo.Dest); serr != nil || !fi.IsDir() {
			continue
		}
		prs := github.ListOpenPRs(repo.Dest)
		for _, pr := range prs {
			sb.WriteString(fmt.Sprintf("#%-4d %-30s %s\n", pr.Number, repo.Repo, pr.URL))
			sb.WriteString(fmt.Sprintf("      branch: %-28s %s\n", pr.Branch, pr.Title))
			total++
		}
	}
	if total == 0 {
		sb.WriteString("no open pull requests\n")
	}
	return sb.String(), nil
}

func walkListNodeToString(sb *strings.Builder, node config.TreeNode, nodeDir, nodePath string, l *config.Loaded) error {
	root, _ := config.ExpandRoot(l.Setup.Workspace.Root)
	wsPrefix := filepath.Join(root, l.DeckName) + "/"

	for _, ref := range node.RepoRefs {
		parts := strings.SplitN(ref, "/", 2)
		repoName := parts[1]
		dest := filepath.Join(nodeDir, repoName)
		prov := "no"
		if fi, err := os.Stat(dest); err == nil && fi.IsDir() {
			prov = "yes"
		}
		rel := strings.TrimPrefix(dest, wsPrefix)
		sb.WriteString(fmt.Sprintf("%-40s %-25s %s\n", rel, ref, prov))
	}

	for _, modName := range node.ModuleRefs {
		mod := l.Modules[modName]
		for _, repo := range mod.Repos {
			dest := filepath.Join(nodeDir, repo)
			prov := "no"
			if fi, err := os.Stat(dest); err == nil && fi.IsDir() {
				prov = "yes"
			}
			rel := strings.TrimPrefix(dest, wsPrefix)
			sb.WriteString(fmt.Sprintf("%-40s %-25s %s\n", rel, modName, prov))
		}
	}

	childNames := sortedStringKeys(node.Children)
	for _, childName := range childNames {
		child := node.Children[childName]
		childPath := childName
		if nodePath != "" {
			childPath = nodePath + "/" + childName
		}
		if err := walkListNodeToString(sb, child, filepath.Join(nodeDir, childName), childPath, l); err != nil {
			return err
		}
	}
	return nil
}

func sortedStringKeys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
