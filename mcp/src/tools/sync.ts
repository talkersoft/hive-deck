import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_sync",
    `Pull the latest changes for every repo in a deck.
Verifies ALL repos are clean (committed, pushed, no stash) before touching anything — aborts if any are dirty.
Run hv_status first if unsure whether the workspace is clean.`,
    {
      deck: z.string().describe("Deck name to sync (e.g. 'cloud-manager'). Use hv_decks to list available decks."),
    },
    async ({ deck }) => {
      const { ok: success, output } = runHv({ op: "sync", deck });
      if (!success) {
        const hint = [
          "Sync aborted — one or more repos are not clean. Next steps:",
          "1. Call hv_status to identify which repos are dirty and why.",
          "2. If repos have unpushed commits → call hv_pr to open pull requests.",
          "3. If repos have uncommitted changes → commit and push them first.",
          "4. Once all repos are clean, retry hv_sync.",
        ].join("\n");
        return err(output, hint);
      }
      return ok(output, "All repos are up to date. You can now start work or call hv_status to verify state.");
    }
  );
}
