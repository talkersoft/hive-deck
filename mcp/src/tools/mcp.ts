import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_mcp",
    `Write MCP server config to {decks_root}/.claude/settings.json for a deck.
Resolves the MCPs listed in the deck file against mcps.yaml and merges an mcpServers block into the workspace settings.
All other keys in that file are preserved. Also runs automatically at the end of hv_init when mcp_manager.enabled is true.
Requires mcp_manager.enabled: true in config.yaml.`,
    {
      deck: z.string().describe("Deck name to apply MCP config for (e.g. 'cloud-manager'). Use hv_decks to list available decks."),
    },
    async ({ deck }) => {
      const { ok: success, output } = runHv({ op: "mcp", deck });
      if (!success) {
        const hint = [
          "MCP config write failed. Common causes:",
          "- mcp_manager.enabled is false in config.yaml — flip it to true.",
          "- An MCP name in the deck file is not defined in mcps.yaml.",
          "- decks_root is not set or the .claude/ directory is not writable.",
        ].join("\n");
        return err(output, hint);
      }
      return ok(output, "MCP server config written to workspace settings.json. Restart Claude Code to pick up the new servers.");
    }
  );
}
