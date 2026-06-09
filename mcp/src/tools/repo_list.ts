import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_repo_list",
    `List repos, modules, or nodes for a deck.
- target="repos": every repo in the deck with its node path and whether it is cloned on disk
- target="modules": all modules defined in modules.yaml and their repos
- target="nodes": all valid node paths to use as the node argument in hv_repo_add`,
    {
      deck: z.string().describe("Deck name (e.g. 'cloud-manager'). Use hv_decks to list available decks."),
      target: z.enum(["repos", "modules", "nodes"]).describe(
        "What to list: 'repos' for all repos, 'modules' for module definitions, 'nodes' for valid node paths."
      ),
    },
    async ({ deck, target }) => {
      const { ok: success, output } = runHv({ op: "repo_list", deck, target });
      return success ? ok(output) : err(output);
    }
  );
}
