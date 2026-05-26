import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_branch",
    `Create a new branch across every repo in an already-provisioned deck.
All repos must be on the default branch and fully clean before this runs — call hv_default first if not.
Use this to start a new task on a workspace that is already provisioned, without re-running hv_init.
Aborts before touching anything if the branch already exists in ANY repo — choose a unique branch name.`,
    {
      deck: z.string().describe("Deck name to branch (e.g. 'cloud-manager'). Use hv_decks to list available decks."),
      branch: z.string().describe("Feature branch name to create across all repos. Use a short evocative 'sillyname' mixing the task theme with a fun word (e.g. 'ansible-shenanigans', 'db-wizard', 'auth-overhaul-spree'). Must not already exist in any repo."),
    },
    async ({ deck, branch }) => {
      const { ok: success, output } = runHv("branch", deck, branch);
      if (!success) {
        const hint = [
          "Branch creation failed. Next steps:",
          "- 'not on default branch' → call hv_default first to reset all repos to main.",
          "- 'branch already exists' → choose a different branch name.",
          "- 'dirty repo' → call hv_status to identify issues, commit and push all changes, then retry.",
          "- 'repos not on disk' → call hv_init instead (provisions and branches in one step).",
        ].join("\n");
        return err(output, hint);
      }
      return ok(output, `All repos are now on branch '${branch}'. Begin work — commit to this branch, then call hv_pr when ready to open pull requests.`);
    }
  );
}
