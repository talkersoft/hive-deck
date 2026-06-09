// Package ops is the JSON CLI backend for hive-deck.
// All operations accept a typed input struct. The Dispatch function
// routes a JSON payload (discriminated by the "op" field) to the
// correct handler. The cobra CLI calls Run* functions directly via
// Go imports; the MCP sends JSON via stdin.
package ops

// StatusInput is the JSON payload for the status operation.
type StatusInput struct {
	Op   string `json:"op"`
	Deck string `json:"deck"`
}

// InitInput is the JSON payload for the init operation.
type InitInput struct {
	Op     string `json:"op"`
	Deck   string `json:"deck"`
	Branch string `json:"branch,omitempty"`
}

// ShipInput is the JSON payload for the ship operation.
type ShipInput struct {
	Op      string `json:"op"`
	Deck    string `json:"deck"`
	Message string `json:"message"`
	Title   string `json:"title"`
	Body    string `json:"body,omitempty"`
}

// SyncInput is the JSON payload for the sync operation.
type SyncInput struct {
	Op   string `json:"op"`
	Deck string `json:"deck"`
}

// PruneInput is the JSON payload for the prune operation.
type PruneInput struct {
	Op     string `json:"op"`
	Deck   string `json:"deck"`
	DryRun bool   `json:"dry_run,omitempty"`
}

// NextInput is the JSON payload for the next operation.
type NextInput struct {
	Op     string `json:"op"`
	Deck   string `json:"deck"`
	Branch string `json:"branch,omitempty"`
}

// StashInput is the JSON payload for the stash push/pop operations.
type StashInput struct {
	Op   string `json:"op"` // "stash_push" or "stash_pop"
	Deck string `json:"deck"`
}

// TeardownInput is the JSON payload for the teardown operation.
type TeardownInput struct {
	Op   string `json:"op"`
	Deck string `json:"deck"`
}

// WorkflowInput is the JSON payload for the workflow operation.
type WorkflowInput struct {
	Op           string `json:"op"`
	Type         string `json:"type,omitempty"` // "workflow" | "plan"; defaults to "workflow"
	Deck         string `json:"deck"`
	WorkflowName string `json:"workflow_name"` // required; names the extension under decks/workflows/<type>/<deck>/
	List         bool   `json:"list,omitempty"`
}

// WorkflowRunInput is the JSON payload for the workflow_run operation.
// Branch is the execution branch name (e.g. "silly-name") — used to locate
// ExecFolder/<deck>/<branch>/Orchestrate/ORCH.md for execution.
type WorkflowRunInput struct {
	Op     string `json:"op"`
	Deck   string `json:"deck"`
	Branch string `json:"branch"`
}

// WorkflowPlanInput is the JSON payload for the workflow_plan operation.
type WorkflowPlanInput struct {
	Op           string `json:"op"`
	Deck         string `json:"deck"`
	WorkflowName string `json:"workflow_name"`
}

// WorkflowListInput is the JSON payload for the workflow_list operation.
type WorkflowListInput struct {
	Op   string `json:"op"`
	Deck string `json:"deck"`
}

// WorkflowCreateInput is the JSON payload for the workflow_create operation.
// Returns scaffold instructions for creating ORCH.md, tasks, and tests in workflow-exec.
type WorkflowCreateInput struct {
	Op           string `json:"op"`
	Deck         string `json:"deck"`
	WorkflowName string `json:"workflow_name"`
}

// DecksInput is the JSON payload for the decks operation.
type DecksInput struct {
	Op string `json:"op"`
}

// ListReposInput is the JSON payload for listing repos in a deck.
type ListReposInput struct {
	Op   string `json:"op"` // "list_repos"
	Deck string `json:"deck"`
}

// ListPullsInput is the JSON payload for listing open pull requests.
type ListPullsInput struct {
	Op   string `json:"op"` // "list_pulls"
	Deck string `json:"deck"`
}

// McpInput is the JSON payload for the mcp operation.
type McpInput struct {
	Op   string `json:"op"`
	Deck string `json:"deck"`
}

// RepoListInput is the JSON payload for the repo_list operation.
type RepoListInput struct {
	Op     string `json:"op"`
	Deck   string `json:"deck"`
	Target string `json:"target"` // "repos" | "modules" | "nodes"
}

// RepoAddInput is the JSON payload for the repo_add operation.
type RepoAddInput struct {
	Op      string `json:"op"`
	Deck    string `json:"deck"`
	Repo    string `json:"repo"`              // "org/repo" matching a known org in config.yaml
	Node    string `json:"node"`              // node path e.g. "" (deck root), "tools", "vm-infra/cloud-manager"
	Module  string `json:"module,omitempty"`  // optional: also add to this module in modules.yaml
	Branch  string `json:"branch,omitempty"`  // defaults to deck branch then "main"
	Message string `json:"message,omitempty"` // initial commit message; default "initial commit"
}

