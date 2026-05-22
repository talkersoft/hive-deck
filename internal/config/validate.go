package config

import (
	"fmt"
	"strings"
)

// ValidateDeck performs pre-flight checks on the loaded deck.
// It walks the deck: tree and verifies every repo and module reference is resolvable.
func (l *Loaded) ValidateDeck() error {
	root := l.DeckFile.Deck
	empty := len(root.Children) == 0 &&
		len(root.RepoRefs) == 0 &&
		len(root.ModuleRefs) == 0 &&
		len(root.Symlinks) == 0
	if empty {
		return fmt.Errorf("deck %q: deck: tree is empty", l.DeckName)
	}
	if l.Setup.DecksRoot == "" {
		return fmt.Errorf("decks_root is not set in ~/.hive/setup.yaml")
	}
	if l.Setup.DefaultBranch == "" {
		return fmt.Errorf("default_branch is not set in ~/.hive/setup.yaml")
	}
	return l.validateNode("", l.DeckFile.Deck)
}

func (l *Loaded) validateNode(nodePath string, node TreeNode) error {
	display := nodePath
	if display == "" {
		display = "<root>"
	}

	// dest tracks names that will land directly in this node's folder, for collision detection.
	dest := map[string]string{} // name → "source description"

	for _, ref := range node.RepoRefs {
		parts := strings.SplitN(ref, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return fmt.Errorf("node %q: invalid repo reference %q, expected org/repo", display, ref)
		}
		orgName, repoName := parts[0], parts[1]
		org, ok := l.Setup.Orgs[orgName]
		if !ok {
			return fmt.Errorf("node %q: repo %q references org %q which is not in setup.yaml", display, ref, orgName)
		}
		if err := validateProtocol(org.Protocol); err != nil {
			return fmt.Errorf("org %q: %w", orgName, err)
		}
		if org.URL == "" {
			return fmt.Errorf("org %q has no url", orgName)
		}
		if prev, conflict := dest[repoName]; conflict {
			return fmt.Errorf("node %q: dest collision for %q between %q and %q", display, repoName, prev, ref)
		}
		dest[repoName] = ref
	}

	for _, modName := range node.ModuleRefs {
		mod, ok := l.Modules[modName]
		if !ok {
			return fmt.Errorf("node %q references unknown module %q (not in modules.yaml)", display, modName)
		}
		if mod.Org == "" {
			return fmt.Errorf("module %q has no org", modName)
		}
		org, ok := l.Setup.Orgs[mod.Org]
		if !ok {
			return fmt.Errorf("module %q references org %q which is not in setup.yaml", modName, mod.Org)
		}
		if err := validateProtocol(org.Protocol); err != nil {
			return fmt.Errorf("org %q: %w", mod.Org, err)
		}
		if org.URL == "" {
			return fmt.Errorf("org %q has no url", mod.Org)
		}
		for _, repo := range mod.Repos {
			if prev, conflict := dest[repo]; conflict {
				return fmt.Errorf("node %q: dest collision for %q between %q and module %q", display, repo, prev, modName)
			}
			dest[repo] = "module:" + modName
		}
	}

	for _, target := range node.Symlinks {
		if target == "" {
			return fmt.Errorf("node %q: symlinks entry must not be empty", display)
		}
	}

	for childName, child := range node.Children {
		if childName == LaunchDir {
			return fmt.Errorf("folder name %q is reserved (collides with the shared hive-workspace/ folder)", childName)
		}
		childPath := childName
		if nodePath != "" {
			childPath = nodePath + "/" + childName
		}
		if err := l.validateNode(childPath, child); err != nil {
			return err
		}
	}
	return nil
}

func validateProtocol(p string) error {
	switch p {
	case "ssh", "https":
		return nil
	default:
		return fmt.Errorf("protocol must be ssh or https, got %q", p)
	}
}
