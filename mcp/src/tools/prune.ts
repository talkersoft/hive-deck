import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_prune",
    `Remove on-disk repos that are no longer declared in the deck YAML.
Scans the deck directory for git repos not listed in the deck, verifies they are all clean, then removes them.
Always regenerates the .code-workspace file — even if nothing was removed — so removing a repo from the deck YAML is immediately reflected.
Use dry_run to preview what would be removed without removing anything.`,
    {
      deck: z.string().describe("Deck name to prune (e.g. 'cloud-manager'). Use hv_decks to list available decks."),
      dry_run: z.boolean().optional().describe("Preview what would be removed without removing anything. Defaults to false."),
    },
    async ({ deck, dry_run }) => {
      const args = ["prune", deck];
      if (dry_run) args.push("--dry-run");
      const { ok: success, output } = runHv(...args);
      if (!success) {
        const hint = [
          "Prune aborted — one or more undeclared repos have uncommitted or unpushed work. Next steps:",
          "1. Call hv_status to identify which repos are dirty.",
          "2. Commit and push any work you want to keep, or discard it manually.",
          "3. Retry hv_prune once all undeclared repos are clean.",
        ].join("\n");
        return err(output, hint);
      }
      return ok(output, "Prune complete. The .code-workspace file has been regenerated.");
    }
  );
}
