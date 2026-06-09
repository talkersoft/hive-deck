import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_stash",
    `Stash uncommitted changes across all repos that have a merged PR — deadlock escape hatch only.
Use this ONLY when a branch has a merged PR and uncommitted changes are blocking hv_next.
Every dirty repo must have a merged or closed PR — repos without one are shippable and hv_ship should be used instead.
After stashing: call hv_next to transition to a new branch, then hv_unstash to restore the changes.`,
    {
      deck: z.string().describe("Deck name (e.g. 'cloud-manager'). Use hv_decks to list available decks."),
    },
    async ({ deck }) => {
      const { ok: success, output } = runHv({ op: "stash_push", deck });
      if (!success) {
        const hint = [
          "Stash failed. Common causes:",
          "- Repos have no merged PR → use hv_ship instead to commit and push normally.",
          "- Repos have an open PR → merge it first, then stash if needed.",
          "Call hv_status to check the current state of each repo.",
        ].join("\n");
        return err(output, hint);
      }
      const hint = "Changes stashed. Call hv_next with a new branch name to transition, then hv_unstash to restore your work onto the new branch.";
      return ok(output, hint);
    }
  );
}
