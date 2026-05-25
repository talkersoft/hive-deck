import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_pr",
    `Open a pull request for every repo in a deck that is on a feature branch with commits ahead of origin/<default>.
The same title and body are applied to every PR created. PR URLs are printed at the end.
Pre-flight checks (aborts before creating any PRs if any fail):
  - Dirty working tree on any repo → abort
  - On default branch with unpushed commits → abort (create a feature branch first)
Skips repos that are: on default branch and clean, on feature branch with nothing new, or already have an open PR.
Requires gh CLI to be authenticated.`,
    {
      deck: z.string().describe("Deck name to create PRs for (e.g. 'cloud-manager'). Use hv_decks to list available decks."),
      title: z.string().describe("PR title applied to every pull request created."),
      body: z.string().optional().describe("PR body/description applied to every pull request. Can be left empty."),
    },
    async ({ deck, title, body }) => {
      const args = ["pr", deck, "--title", title];
      if (body) args.push("--body", body);
      const { ok: success, output } = runHv(...args);

      if (!success) {
        const hint = [
          "PR creation blocked. Next steps based on the error above:",
          "- 'uncommitted changes' → commit the changes in that repo first.",
          "- 'on default branch with unpushed commits' → move this work to a feature branch before opening a PR.",
          "- 'no upstream branch configured' → push the branch with --set-upstream first so gh can target it.",
          "Call hv_status to get a full picture of each repo's state, resolve the issues, then retry hv_pr.",
        ].join("\n");
        return err(output, hint);
      }

      const hint = [
        "Pull requests created successfully.",
        "Next steps:",
        "- Review and merge the PRs at the URLs listed above.",
        "- Once merged, call hv_default to switch all repos back to their default branch and pull the merged changes.",
      ].join("\n");
      return ok(output, hint);
    }
  );
}
