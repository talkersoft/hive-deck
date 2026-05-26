import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_decks",
    `List all available deck files in the active hv config directory (~/.hv/).
Each deck name can be passed to other hv_* tools as the 'deck' argument.
Run this when you are unsure what decks are configured on this machine.`,
    {},
    async () => {
      const { ok: success, output } = runHv("list", "decks");
      return success ? ok(output) : err(output);
    }
  );
}
