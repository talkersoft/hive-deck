import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_orchestrate_run",
    `Read the ORCH.md for an existing orchestration and return it for execution.
Pass the branch name produced by hv_orchestrate (e.g. "silly-name").
The agent uses the returned ORCH.md to execute all tasks in order.`,
    {
      deck: z.string().describe(
        "Deck name (e.g. 'hive-deck-pro'). Use hv_decks to list available decks."
      ),
      branch: z.string().describe(
        "Execution branch name produced by hv_orchestrate (e.g. 'jazzy-narwhal'). " +
        "This identifies which orchestration scaffold to execute in workflow-exec."
      ),
    },
    async ({ deck, branch }) => {
      const { ok: success, output } = runHv({ op: "orchestrate_run", deck, branch });
      if (!success) {
        return err(output);
      }
      return ok(output);
    }
  );
}
