import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_orchestrate_list",
    `List all available orchestrations for a deck.
Calls \`hv orchestrate --list <deck>\` and returns the list of orchestration names that can be
passed to hv_orchestrate. Use hv_decks to find valid deck names.`,
    {
      deck: z.string().describe(
        "Deck name (e.g. 'cloud-manager'). Use hv_decks to list available decks."
      ),
    },
    async ({ deck }) => {
      const { ok: success, output } = runHv({ op: "orchestrate_list", deck });
      if (!success) {
        return err(output);
      }
      return ok(output);
    }
  );
}
