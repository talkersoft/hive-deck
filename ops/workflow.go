package ops

import (
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/talkersoft/hive-deck/internal/builtin"
	"github.com/talkersoft/hive-deck/internal/config"
	"github.com/talkersoft/hive-deck/internal/resolve"
	wf "github.com/talkersoft/hive-deck/internal/workflow"
)

func RunWorkflow(in WorkflowInput) (string, error) {
	wfType := in.Type
	if wfType == "" {
		wfType = "workflow"
	}

	l, err := config.LoadDeck(in.Deck)
	if err != nil {
		return "", err
	}

	plan, err := resolve.Build(l)
	if err != nil {
		return "", err
	}
	tokens := map[string]string{
		"DeckRoot": plan.DeckDir,
		"Deck":     l.DeckName,
	}
	for _, r := range plan.Repos {
		if br, berr := repoCurrentBranch(r.Dest); berr == nil {
			tokens["Branch"] = br
			break
		}
	}
	tokens["ExecFolder"] = resolveWorkflowPath(l.Setup.ExecFolder, plan.DeckDir, filepath.Join(plan.DeckDir, "planning", "workflow-exec"))
	tokens["PlanFolder"] = resolveWorkflowPath(l.Setup.PlanFolder, plan.DeckDir, filepath.Join(plan.DeckDir, "planning", "workflow-plans"))

	if in.List {
		return listWorkflowNames(configRoot(), l.DeckName)
	}

	if in.WorkflowName == "" {
		return "", fmt.Errorf("workflow_name is required — use hv promote <deck> <workflowName>")
	}

	// Search order:
	// 1. <configRoot>/decks/workflows/<type>/<deck>/<workflowName>.yaml  (workflow-configuration repo)
	// 2. ~/.hv/workflows/<type>/<deck>/<workflowName>.yaml               (machine-local overrides)
	extBytes, extErr := loadExtensionBytes(configRoot(), wfType, l.DeckName, in.WorkflowName)
	if extErr != nil {
		extBytes, extErr = config.LoadWorkflowExtension(wfType, in.WorkflowName)
	}
	if extErr != nil {
		return "", fmt.Errorf("workflow %q not found for deck %q — create decks/workflows/%s/%s/%s.yaml", in.WorkflowName, l.DeckName, wfType, l.DeckName, in.WorkflowName)
	}
	return runExtensionWorkflow(l, wfType, in.WorkflowName, extBytes, tokens)
}

func runExtensionWorkflow(l *config.Loaded, wfType, name string, extBytes []byte, tokens map[string]string) (string, error) {
	base, err := loadBuiltinBase(wfType)
	if err != nil {
		return "", err
	}
	var assembled *wf.AssembledWorkflow
	switch wfType {
	case "plan":
		var ext wf.PlanExtension
		if err := yaml.Unmarshal(extBytes, &ext); err != nil {
			return "", fmt.Errorf("parse plan extension %q: %w", name, err)
		}
		assembled, err = wf.AssembleFromPlanExtension(base, &ext, tokens)
	default:
		var ext wf.WorkflowExtension
		if err := yaml.Unmarshal(extBytes, &ext); err != nil {
			return "", fmt.Errorf("parse workflow extension %q: %w", name, err)
		}
		assembled, err = wf.AssembleFromExtension(base, &ext, tokens)
	}
	if err != nil {
		return "", err
	}
	return renderAssembled(l, assembled, tokens)
}

func renderAssembled(l *config.Loaded, assembled *wf.AssembledWorkflow, tokens map[string]string) (string, error) {
	data := wf.TokensToData(tokens)
	data.PRMode = string(l.Setup.Ship.PRMode)
	data.DeleteBranchOnMerge = l.Setup.Ship.DeleteBranchOnMerge
	data.Agent = l.DeckFile.Agent
	if data.Agent == "" {
		data.Agent = "claude"
	}
	var sections []string
	for _, task := range assembled.Tasks {
		if task.Source == "user" && task.Title != "" {
			sections = append(sections, fmt.Sprintf("# Task %s — %s", task.Number, task.Title))
		}
		rendered, err := wf.Render(configRoot(), l.DeckName, task.Fragments, data, builtin.FS)
		if err != nil {
			return "", err
		}
		if rendered != "" {
			sections = append(sections, rendered)
		}
	}
	return strings.Join(sections, "\n"), nil
}

// RunWorkflowExecute reads ExecFolder/<deck>/<branch>/Orchestrate/ORCH.md
// and returns its content for the agent to execute.
func RunWorkflowExecute(in WorkflowRunInput) (string, error) {
	l, err := config.LoadDeck(in.Deck)
	if err != nil {
		return "", err
	}
	plan, err := resolve.Build(l)
	if err != nil {
		return "", err
	}
	execFolder := resolveWorkflowPath(l.Setup.ExecFolder, plan.DeckDir, filepath.Join(plan.DeckDir, "planning", "workflow-exec"))
	orchPath := filepath.Join(execFolder, l.DeckName, in.Branch, "Orchestrate", "ORCH.md")
	b, err := os.ReadFile(orchPath)
	if err != nil {
		return "", fmt.Errorf("ORCH.md not found at %s — has the workflow been created yet?", orchPath)
	}
	return string(b), nil
}

func loadBuiltinBase(wfType string) (wf.BuiltinBase, error) {
	path := wfType + "/_base.yaml"
	b, err := fs.ReadFile(builtin.FS, path)
	if err != nil {
		return wf.BuiltinBase{}, fmt.Errorf("builtin base for %q: %w", wfType, err)
	}
	var base wf.BuiltinBase
	if err := yaml.Unmarshal(b, &base); err != nil {
		return wf.BuiltinBase{}, fmt.Errorf("parse builtin base: %w", err)
	}
	return base, nil
}

// configRoot returns the directory containing config.yaml.
// HV_HOME points directly at it; otherwise falls back to ~/.hv/.
func configRoot() string {
	if h := os.Getenv("HV_HOME"); h != "" {
		return h
	}
	if home, err := os.UserHomeDir(); err == nil {
		return filepath.Join(home, config.ConfigDir)
	}
	return ""
}

func loadExtensionBytes(root, wfType, deck, workflowName string) ([]byte, error) {
	if root == "" || workflowName == "" {
		return nil, fmt.Errorf("no config root or workflow name")
	}
	// decks/workflows/<type>/<deck>/<workflowName>.yaml
	p := filepath.Join(root, "decks", "workflows", wfType, deck, workflowName+".yaml")
	if b, err := os.ReadFile(p); err == nil {
		return b, nil
	}
	return nil, fmt.Errorf("not found: %s", p)
}

// resolveWorkflowPath expands {{DeckRoot}} in a config-supplied path.
// Falls back to defaultPath when the config value is empty.
func resolveWorkflowPath(configured, deckRoot, defaultPath string) string {
	if configured == "" {
		return defaultPath
	}
	return strings.ReplaceAll(configured, "{{DeckRoot}}", deckRoot)
}

func listWorkflowNames(root, deck string) (string, error) {
	var sb strings.Builder
	if root == "" {
		return sb.String(), nil
	}
	listExtensions(&sb, filepath.Join(root, "decks", "workflows", "workflow", deck), "workflow")
	listExtensions(&sb, filepath.Join(root, "decks", "workflows", "plan", deck), "plan")
	return sb.String(), nil
}

func listExtensions(sb *strings.Builder, dir, kind string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".yaml") {
			name := strings.TrimSuffix(e.Name(), ".yaml")
			sb.WriteString(fmt.Sprintf("  %-10s ", kind))
			sb.WriteString(name)
			sb.WriteByte('\n')
		}
	}
}

func repoCurrentBranch(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	b := strings.TrimSpace(string(out))
	if b == "HEAD" {
		return "", fmt.Errorf("detached HEAD in %s", dir)
	}
	return b, nil
}
