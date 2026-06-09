import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_teardown",
    `Surgically remove tracked files and .git/ from every repo in a deck, leaving untracked files intact.
Refuses to run if ANY repo has uncommitted changes, unpushed commits, detached HEAD, or stash entries.
This is the completion signal in agentic workflows — a successful teardown means all work is committed and pushed.
The workspace can be fully restored with hv_provision.
WARNING: This removes source files. Only run when all work is committed and pushed.`,
    {
      deck: z.string().describe("Deck name to tear down (e.g. 'cloud-manager'). Use hv_decks to list available decks."),
    },
    async ({ deck }) => {
      const { ok: success, output } = runHv({ op: "teardown", deck });

      if (!success) {
        const hint = [
          "Teardown refused — work would be lost. The workspace has NOT been modified. Next steps:",
          "1. Call hv_status to see exactly which repos are blocking teardown and why.",
          "2. If repos have unpushed commits → call hv_pr to open pull requests, or push manually.",
          "3. If repos have uncommitted changes → commit and push them.",
          "4. Once hv_status shows all repos clean, retry hv_teardown.",
        ].join("\n");
        return err(output, hint);
      }

      return ok(output, "Teardown complete. All tracked files removed, untracked files preserved. Call hv_provision to restore the workspace when ready.");
    }
  );
}
