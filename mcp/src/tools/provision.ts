import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_provision",
    `Provision every repo in a deck — idempotent, safe to run multiple times.
For each declared repo:
  - Not cloned → clones it
  - Torn-down shell (no .git/) → restores in place, preserving untracked files
  - Already cloned → skipped
Also creates any GitHub repos that do not yet exist (requires gh CLI auth).
Generates a VS Code .code-workspace file after provisioning.`,
    {
      deck: z.string().describe("Deck name to provision (e.g. 'cloud-manager'). Use hv_decks to list available decks."),
      filter: z.string().optional().describe("Only provision repos under this node (e.g. 'tools'). Omit to provision the full deck."),
    },
    async ({ deck, filter }) => {
      const args = ["provision", deck];
      if (filter) args.push("--filter", filter);
      const { ok: success, output } = runHv(...args);
      if (!success) {
        const hint = [
          "Provision failed. Common causes:",
          "- Clone failed → check network access and that gh CLI is authenticated (run: gh auth status).",
          "- GitHub repo creation failed → ensure the org/repo name in the deck file is correct.",
          "Retry hv_provision after resolving the issue — it is idempotent and safe to re-run.",
        ].join("\n");
        return err(output, hint);
      }
      return ok(output, "Provision complete. Call hv_status to verify all repos are clean and ready.");
    }
  );
}
