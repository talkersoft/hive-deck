import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

function summarize(deck: string, output: string): string {
  const lines = output.split("\n");

  // Extract shipped repos
  const shipped: string[] = [];
  for (const line of lines) {
    const m = line.match(/^\s+ship\s+(\S+)/);
    if (m) shipped.push(m[1]);
  }

  // Extract PRs: track url and whether merged per repo
  const prs: Record<string, { url: string; merged: boolean }> = {};
  for (const line of lines) {
    const pr = line.match(/^\s+pr\s+(\S+)\s+(https:\/\/\S+)/);
    if (pr) prs[pr[1]] = { url: pr[2], merged: false };
    const merged = line.match(/^\s+merged\s+(\S+)\s+(https:\/\/\S+)/);
    if (merged) {
      const num = merged[2].match(/\/(\d+)$/)?.[1];
      prs[merged[1]] = { url: merged[2], merged: true };
      if (num) prs[merged[1]].url = `PR #${num}  ${merged[2]}`;
    }
  }
  // Reformat pr urls to include PR number
  for (const repo of Object.keys(prs)) {
    const num = prs[repo].url.match(/\/(\d+)$/)?.[1];
    if (num && !prs[repo].url.startsWith("PR #")) {
      prs[repo].url = `PR #${num}  ${prs[repo].url}`;
    }
  }

  // Extract next branch
  const nextMatch = output.match(/^next: \S+ → (\S+)/m);
  const nextBranch = nextMatch?.[1];

  // Extract current (from) branch — first line: "ship: <deck> repos=N"
  const fromMatch = output.match(/^ship: \S+/m);

  const mergeGate = output.includes("\nmerge-gate:");
  const awaitMerge = output.includes("\nawait_merge:");
  const autoMerged = output.includes("\nauto-merged:");
  const teardown = output.includes("teardown:");

  const parts: string[] = [];

  // Header
  parts.push(`Shipped: ${deck}`);
  parts.push("");

  if (shipped.length === 0) {
    parts.push("  (nothing to ship — all repos already clean)");
  } else {
    for (const repo of shipped) {
      const pr = prs[repo];
      if (pr) {
        const status = pr.merged ? "merged" : (mergeGate || awaitMerge) ? "open (pending merge)" : "open";
        parts.push(`  ${repo.padEnd(28)} ${pr.url}  ${status}`);
      } else {
        parts.push(`  ${repo.padEnd(28)} pushed`);
      }
    }
  }

  parts.push("");

  if (nextBranch) {
    parts.push(`Next branch: ${nextBranch}`);
    if (teardown) {
      parts.push("Repos torn down from disk. Run hv_init to start the next task.");
    } else if (autoMerged) {
      parts.push("All PRs merged. Run hv_next or hv_init to start the next task.");
    } else if (mergeGate) {
      parts.push("PRs open — merge them, then run hv_next to advance.");
    } else {
      parts.push("Run hv_next or hv_init to start the next task.");
    }
  }

  return parts.join("\n");
}

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
      const { ok: success, output } = runHv({ op: "ship", deck, message, title, ...(body ? { body } : {}) });
      if (!success) {
        const hint = [
          "Ship failed. Common causes:",
          "- On default branch → call hv_branch or hv_init to create a feature branch first.",
          "- Push rejected → remote has changes not present locally; call hv_sync first.",
          "- Dirty repo blocked commit → call hv_status to identify the issue.",
        ].join("\n");
        return err(output, hint);
      }
      const summary = summarize(deck, output);
      const hint = `[agent: print every PR URL from the output above directly to the user — do not skip this step]`;
      const awaitMergeMode = output.includes("\nawait_merge:");
      if (awaitMergeMode) {
        return ok(summary, `PRs are open and waiting to be merged. hv_ship is polling — it will transition automatically once all PRs are merged.\n${hint}`);
      }
      return ok(summary, hint);
    }
  );
}
