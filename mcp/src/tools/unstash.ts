import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_unstash",
    `Restore stashed changes onto the current branch across all repos that have a stash entry.
Use after hv_next to bring work stashed on the previous branch onto the new one.`,
    {
      deck: z.string().describe("Deck name (e.g. 'cloud-manager'). Use hv_decks to list available decks."),
    },
    async ({ deck }) => {
      const { ok: success, output } = runHv("stash", "pop", deck);
      if (!success) {
        const hint = "Unstash failed — a stash conflict may have occurred. Check git status on the affected repos and resolve conflicts manually.";
        return err(output, hint);
      }
      const hint = "Stashed changes restored. You are back on a clean feature branch with your work in place. Proceed as normal — call hv_ship when ready.";
      return ok(output, hint);
    }
  );
}
