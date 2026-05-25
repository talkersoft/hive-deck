import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_default",
    `Switch every repo in a deck to its default branch (main/master) and pull.
Verifies ALL repos are fully clean and pushed on their current branch before switching anything.
Use this to reset a workspace after finishing a feature — ensures no repo is left on a stale branch.
Safe to run even if repos are already on the default branch (they will just be pulled).`,
    {
      deck: z.string().describe("Deck name to reset to default branches (e.g. 'cloud-manager'). Use hv_decks to list available decks."),
      filter: z.string().optional().describe("Only reset repos under this node. Omit to reset the full deck."),
    },
    async ({ deck, filter }) => {
      const args = ["default", deck];
      if (filter) args.push("--filter", filter);
      const { ok: success, output } = runHv(...args);

      if (!success) {
        const hint = [
          "Cannot reset — one or more repos are not in a clean state. Next steps:",
          "1. Call hv_status to see exactly which repos are dirty and why.",
          "2. If repos have unpushed commits on a feature branch → call hv_pr to open pull requests first.",
          "3. If repos have uncommitted changes → those must be committed and pushed before resetting.",
          "4. Once all repos are clean and pushed, retry hv_default.",
        ].join("\n");
        return err(output, hint);
      }

      const hint = [
        "All repos are now on their default branch and up to date.",
        "Next steps depend on your goal:",
        "- Starting new work → create a feature branch in the relevant repos and begin.",
        "- Done for the session → call hv_status to confirm everything is clean.",
        "- Need latest from remote → hv_sync will pull any remaining updates.",
      ].join("\n");
      return ok(output, hint);
    }
  );
}
