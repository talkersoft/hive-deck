import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_init",
    `Provision every repo in a deck and create a feature branch across all of them.
Idempotent for provisioning: missing repos are cloned, torn-down shells are restored, already-cloned repos are skipped.
After provisioning, every repo is checked out on the specified branch (created fresh from default).
For already-provisioned repos: requires all repos are on the default branch, clean, and the branch does not yet exist in any repo.
GitHub create-if-missing is always on — repos that don't exist on GitHub are created before cloning.
The branch name is always chosen by the AI — never ask the user for one. Pick a short, whimsical, memorable name with no relation to the task (e.g. 'velvet-thunder', 'cosmic-hamster', 'silver-mango'). Do NOT use task-themed names like 'ansible-shenanigans' or 'auth-overhaul'.`,
    {
      deck: z.string().describe("Deck name to initialize (e.g. 'cloud-manager'). Use hv_decks to list available decks."),
      branch: z.string().describe("Feature branch name — always chosen by the AI, never the user. Pick something whimsical and unrelated to the task (e.g. 'velvet-thunder', 'cosmic-hamster', 'silver-mango'). Must not already exist in any repo."),
    },
    async ({ deck, branch }) => {
      const { ok: success, output } = runHv("init", deck, branch);
      if (!success) {
        const hint = [
          "Init failed. Common causes:",
          "- Repos are dirty → call hv_status to identify issues, commit and push all changes first.",
          "- Branch already exists in one or more repos → choose a different branch name.",
          "- Clone failed → check network access and gh CLI auth (run: gh auth status).",
          "Call hv_status to see the current state of each repo, then retry hv_init.",
        ].join("\n");
        return err(output, hint);
      }
      return ok(output, `Workspace ready on branch '${branch}'. All repos are provisioned and on this branch. Begin work — commit to this branch, then call hv_pr when ready to open pull requests.`);
    }
  );
}
