package deckfile

import (
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// AddRepoToDeckFile edits the deck YAML at deckPath, appending repoRef
// under the repos: sequence at the given nodePath.
// nodePath is slash-separated ("" = deck root, "tools", "vm-infra/cloud-manager").
// Idempotent: no-op if repoRef already present.
func AddRepoToDeckFile(deckPath, nodePath, repoRef string) error {
	b, err := os.ReadFile(deckPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", deckPath, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("parse %s: %w", deckPath, err)
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return fmt.Errorf("empty document: %s", deckPath)
	}
	root := doc.Content[0] // MappingNode

	// Navigate to "deck:" key
	deckNode := findInMapping(root, "deck")
	if deckNode == nil {
		return fmt.Errorf("no deck: key in %s", deckPath)
	}

	// Navigate node path parts
	current := deckNode
	if nodePath != "" {
		for _, part := range strings.Split(nodePath, "/") {
			child := findInMapping(current, part)
			if child == nil {
				child = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
				current.Content = append(current.Content,
					&yaml.Node{Kind: yaml.ScalarNode, Value: part, Tag: "!!str"},
					child,
				)
			}
			current = child
		}
	}

	// Find or create repos:
	reposNode := findInMapping(current, "repos")
	if reposNode == nil {
		reposNode = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		current.Content = append(current.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: "repos", Tag: "!!str"},
			reposNode,
		)
	}

	// Idempotency check
	for _, item := range reposNode.Content {
		if item.Value == repoRef {
			return nil
		}
	}

	reposNode.Content = append(reposNode.Content, &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: repoRef,
		Tag:   "!!str",
	})

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", deckPath, err)
	}
	return os.WriteFile(deckPath, out, 0o644)
}

// AddRepoToModule edits modules.yaml at modulesPath, appending repoName
// under the given module's repos: sequence.
// If the module doesn't exist it is created with the given orgName.
// Idempotent: no-op if repoName already present.
func AddRepoToModule(modulesPath, moduleName, orgName, repoName string) error {
	b, err := os.ReadFile(modulesPath)
	if err != nil {
		return fmt.Errorf("read %s: %w", modulesPath, err)
	}
	var doc yaml.Node
	if err := yaml.Unmarshal(b, &doc); err != nil {
		return fmt.Errorf("parse %s: %w", modulesPath, err)
	}
	if doc.Kind == 0 || len(doc.Content) == 0 {
		return fmt.Errorf("empty document: %s", modulesPath)
	}
	root := doc.Content[0]

	modNode := findInMapping(root, moduleName)
	if modNode == nil {
		// Create new module entry
		reposSeq := &yaml.Node{
			Kind: yaml.SequenceNode, Tag: "!!seq",
			Content: []*yaml.Node{{Kind: yaml.ScalarNode, Value: repoName, Tag: "!!str"}},
		}
		modNode = &yaml.Node{
			Kind: yaml.MappingNode, Tag: "!!map",
			Content: []*yaml.Node{
				{Kind: yaml.ScalarNode, Value: "org", Tag: "!!str"},
				{Kind: yaml.ScalarNode, Value: orgName, Tag: "!!str"},
				{Kind: yaml.ScalarNode, Value: "repos", Tag: "!!str"},
				reposSeq,
			},
		}
		root.Content = append(root.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: moduleName, Tag: "!!str"},
			modNode,
		)
	} else {
		reposNode := findInMapping(modNode, "repos")
		if reposNode == nil {
			reposNode = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
			modNode.Content = append(modNode.Content,
				&yaml.Node{Kind: yaml.ScalarNode, Value: "repos", Tag: "!!str"},
				reposNode,
			)
		}
		for _, item := range reposNode.Content {
			if item.Value == repoName {
				return nil // idempotent
			}
		}
		reposNode.Content = append(reposNode.Content, &yaml.Node{
			Kind: yaml.ScalarNode, Value: repoName, Tag: "!!str",
		})
	}

	out, err := yaml.Marshal(&doc)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", modulesPath, err)
	}
	return os.WriteFile(modulesPath, out, 0o644)
}

// CollectNodes walks a deck TreeNode and returns all node paths (slash-separated).
// The deck root is represented as "".
func CollectNodes(nodePath string, children map[string]interface{}) []string {
	// This is handled in the ops layer via config.TreeNode walking.
	return nil
}

func findInMapping(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}
