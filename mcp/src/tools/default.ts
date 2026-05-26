import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_next",
    `Transition every repo in the deck to a new feature branch based on origin/<default>.
Verifies ALL repos are clean, pushed, and (if require_merged_pr is on) PRs merged before transitioning.
Fetches origin and creates <branch> from origin/<default> — the local default branch is never checked out.
Refuses if <branch> equals the default branch name (main/master).
Use this after shipping to start the next task without ever landing on main.`,
    {
      deck: z.string().describe("Deck name (e.g. 'cloud-manager'). Use hv_decks to list available decks."),
      branch: z.string().describe("Name of the new feature branch — always chosen by the AI, never the user. Pick something whimsical and unrelated to the task (e.g. 'velvet-thunder', 'cosmic-hamster', 'silver-mango'). Must not be main/master."),
    },
    async ({ deck, branch }) => {
      const { ok: success, output } = runHv("next", deck, branch);

      if (!success) {
        const hint = [
          "Cannot transition — one or more repos are not ready. Next steps:",
          "1. Call hv_status to see which repos are dirty and why.",
          "2. If repos have unpushed commits or no PR → call hv_ship first.",
          "3. If an open PR is blocking (require_merged_pr: true) → merge it first.",
          "4. If the branch name equals the default branch → choose a different name.",
        ].join("\n");
        return err(output, hint);
      }

      const hint = `All repos are now on '${branch}' based on origin/main. Ready to start the next task — make changes and call hv_ship when done.`;
      return ok(output, hint);
    }
  );
}
