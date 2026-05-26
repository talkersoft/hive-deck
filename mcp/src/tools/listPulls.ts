import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_list_pulls",
    `Show all open pull requests across every provisioned repo in the deck.
Use this to see which repos have outstanding PRs before transitioning branches or tearing down.
Returns PR number, repo, branch, title, and URL for each open PR.`,
    {
      deck: z.string().describe("Deck name (e.g. 'cloud-manager'). Use hv_decks to list available decks."),
    },
    async ({ deck }) => {
      const { ok: success, output } = runHv("list", "pulls", deck);
      if (!success) {
        return err(output);
      }
      const hint = output.includes("no open pull requests")
        ? "No open PRs — all repos are clean. Safe to transition or teardown."
        : "Review open PRs above. Merge or close them before tearing down, or use hv_next if PRs are already merged.";
      return ok(output, hint);
    }
  );
}
