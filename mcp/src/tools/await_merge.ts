import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_await_merge",
    `Check once whether all open PRs on the deck are merged.
- Returns "pending: N PR(s) open" if PRs are still open.
- Returns "merged: all PRs merged → transitioned to <branch>" when all are merged and the deck has auto-transitioned.
This tool checks once and returns immediately — it does not block or sleep.
It is designed to be called from a /loop, not directly.`,
    {
      deck: z.string().describe(
        "Deck name (e.g. 'cloud-manager'). Use hv_decks to list available decks."
      ),
    },
    async ({ deck }) => {
      const { ok: success, output } = runHv({ op: "await_merge", deck });
      if (!success) return err(output);
      const done = output.startsWith("merged:");
      return ok(
        output,
        done
          ? "All PRs merged and branch transitioned. Loop complete — stop the background task."
          : "PRs still open. Loop will check again shortly."
      );
    }
  );
}
