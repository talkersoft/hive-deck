import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_status",
    `Report the git state of every repo in a deck. Shows clean/dirty status with reasons for each repo.
Run this first before any other hv operation to understand the current workspace state.
A repo is clean only when: working tree committed, everything pushed to upstream, no stash, upstream configured.
Dirty reasons include: uncommitted changes, unpushed commits, detached HEAD, stash entries.`,
    {
      deck: z.string().describe("Deck name to check (e.g. 'cloud-manager'). Use hv_decks to list available decks."),
      filter: z.string().optional().describe("Only show repos under this node name (e.g. 'tools', 'vm-infra/cloud-manager')."),
    },
    async ({ deck, filter }) => {
      const args = ["status", deck];
      if (filter) args.push("--filter", filter);
      const { ok: success, output } = runHv(...args);

      if (!success) {
        return err(output, "hv_status failed to run. Check that the deck name is correct using hv_decks.");
      }

      const hasDirty = output.includes("DIRTY");
      const hasMissing = output.includes("MISSING");

      if (!hasDirty && !hasMissing) {
        return ok(output, "All repos are clean. Safe to run hv_sync, hv_default, hv_pr, or hv_teardown.");
      }

      const hints: string[] = ["Some repos need attention before running action commands:"];
      if (hasDirty) {
        hints.push(
          "- Repos marked DIRTY have one or more issues (see reasons listed under each):",
          "    'uncommitted changes' → changes need to be staged and committed in that repo.",
          "    'not pushed to upstream' → commits exist locally but not on remote; call hv_pr to open a PR, or push manually.",
          "    'no upstream branch configured' → the branch has never been pushed; push with --set-upstream first.",
          "    'stash entries present' → stashed work exists; pop or drop the stash.",
          "    'detached HEAD' → not on a branch; check out a branch before proceeding.",
        );
      }
      if (hasMissing) {
        hints.push("- Repos marked MISSING are not cloned. Call hv_provision to clone them.");
      }
      hints.push("Once all issues are resolved, re-run hv_status to confirm before proceeding.");

      return ok(output, hints.join("\n"));
    }
  );
}
