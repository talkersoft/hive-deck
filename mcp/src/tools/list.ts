import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_list",
    `List every repo declared in a deck with its provisioned state (yes/no).
Use this to see what repos belong to a deck and whether they are cloned on disk.
Useful before provision to understand what will be cloned.`,
    {
      deck: z.string().describe("Deck name to list (e.g. 'cloud-manager'). Use hv_decks to list available decks."),
    },
    async ({ deck }) => {
      const { ok: success, output } = runHv({ op: "list_repos", deck });
      const hint = output.includes("no")
        ? "Some repos are not provisioned. Run hv_provision to clone them."
        : undefined;
      return success ? ok(output, hint) : err(output);
    }
  );
}
