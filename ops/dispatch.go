package ops

import (
	"encoding/json"
	"fmt"
)

// Dispatch routes a JSON payload to the correct operation handler.
// The payload must contain an "op" field identifying the operation.
// See ops/contract/CONTRACT.md for the full input specification.
func Dispatch(payload []byte) (string, error) {
	var base struct {
		Op string `json:"op"`
	}
	if err := json.Unmarshal(payload, &base); err != nil {
		return "", fmt.Errorf("invalid JSON payload: %w", err)
	}

	switch base.Op {
	case "status":
		var in StatusInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("status: %w", err)
		}
		return RunStatus(in)

	case "init":
		var in InitInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("init: %w", err)
		}
		return RunInit(in)

	case "ship":
		var in ShipInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("ship: %w", err)
		}
		return RunShip(in)

	case "sync":
		var in SyncInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("sync: %w", err)
		}
		return RunSync(in)

	case "prune":
		var in PruneInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("prune: %w", err)
		}
		return RunPrune(in)

	case "next":
		var in NextInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("next: %w", err)
		}
		return RunNext(in)

	case "stash_push":
		var in StashInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("stash_push: %w", err)
		}
		return RunStashPush(in)

	case "stash_pop":
		var in StashInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("stash_pop: %w", err)
		}
		return RunStashPop(in)

	case "teardown":
		var in TeardownInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("teardown: %w", err)
		}
		return RunTeardown(in)

	case "orchestrate", "promote":
		var in WorkflowInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("promote: %w", err)
		}
		return RunWorkflow(in)

	case "orchestrate_run":
		var in WorkflowRunInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("orchestrate_run: %w", err)
		}
		return RunWorkflowExecute(in)

	case "orchestrate_create":
		var in WorkflowCreateInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("orchestrate_create: %w", err)
		}
		return RunWorkflow(WorkflowInput{Op: "orchestrate", Type: "workflow", Deck: in.Deck, WorkflowName: in.WorkflowName})

	case "orchestrate_plan":
		var in WorkflowPlanInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("orchestrate_plan: %w", err)
		}
		return RunWorkflow(WorkflowInput{Op: "orchestrate", Type: "plan", Deck: in.Deck, WorkflowName: in.WorkflowName})

	case "orchestrate_list":
		var in WorkflowListInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("orchestrate_list: %w", err)
		}
		return RunWorkflow(WorkflowInput{Op: "orchestrate", Type: "workflow", Deck: in.Deck, List: true})

	case "decks":
		var in DecksInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("decks: %w", err)
		}
		return RunDecks(in)

	case "list_repos":
		var in ListReposInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("list_repos: %w", err)
		}
		return RunListRepos(in)

	case "list_pulls":
		var in ListPullsInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("list_pulls: %w", err)
		}
		return RunListPulls(in)

	case "mcp":
		var in McpInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("mcp: %w", err)
		}
		return RunMcp(in)

	case "await_merge":
		var in AwaitMergeInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("await_merge: %w", err)
		}
		return RunAwaitMerge(in)

	case "repo_list":
		var in RepoListInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("repo_list: %w", err)
		}
		return RunRepoList(in)

	case "repo_add":
		var in RepoAddInput
		if err := json.Unmarshal(payload, &in); err != nil {
			return "", fmt.Errorf("repo_add: %w", err)
		}
		return RunRepoAdd(in)

	default:
		return "", fmt.Errorf("unknown op %q — see ops/contract/CONTRACT.md for valid operations", base.Op)
	}
}
