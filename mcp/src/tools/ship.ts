import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_ship",
    `Commit, push, and open pull requests for every repo in the deck — all in one step.
Per repo: uncommitted changes → git add -A + commit + push. Committed but unpushed → push only. Nothing to do → skip.
After pushing, opens a PR for every repo on a feature branch ahead of origin/<default>.
Skips repos that are already clean, have nothing ahead of default, or already have an open PR.
Refuses to run if any repo is on the default branch — only ships from feature branches.
Automatically sets upstream tracking on first push.`,
    {
      deck: z.string().describe("Deck name to ship (e.g. 'cloud-manager'). Use hv_decks to list available decks."),
      message: z.string().describe("Commit message applied to every repo that has uncommitted changes."),
      title: z.string().describe("PR title applied to every pull request created."),
      body: z.string().optional().describe("PR body/description applied to every pull request. Can be left empty."),
    },
    async ({ deck, message, title, body }) => {
      const args = ["ship", deck, message, "--title", title];
      if (body) args.push("--body", body);
      const { ok: success, output } = runHv(...args);
      if (!success) {
        const hint = [
          "Ship failed. Common causes:",
          "- On default branch → call hv_branch or hv_init to create a feature branch first.",
          "- Push rejected → remote has changes not present locally; call hv_sync first.",
          "- Dirty repo blocked commit → call hv_status to identify the issue.",
        ].join("\n");
        return err(output, hint);
      }
      const mergeGate = output.includes("\nmerge-gate:");
      const autoMerged = output.includes("\nauto-merged:");
      const teardownOnShip = output.includes("teardown:");
      const hint = autoMerged && teardownOnShip
        ? "All changes committed, pushed, PRs merged, and repos torn down from disk. Run hv_init with a new branch name to start the next task."
        : autoMerged
        ? "All changes committed, pushed, and PRs merged. Call hv_next with a new branch name to start the next task."
        : mergeGate
        ? "All changes committed, pushed, and PRs opened. require_merged_pr is on — merge them, then call hv_next with a new branch name to transition."
        : "All changes committed and pushed. Call hv_next with a new branch name when ready to start the next task.";
      return ok(output, hint);
    }
  );
}
