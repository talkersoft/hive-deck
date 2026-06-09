import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_repo_add",
    `Add a new repo to a deck and initialize it on GitHub.

Handles three disk states automatically:
- Folder does not exist: creates GitHub repo, clones it into the deck node folder
- Folder exists, no .git/: git init, adds remote, commits existing files, pushes
- Folder exists, has .git/: sets/verifies remote, pushes current state

Always edits the deck YAML to add the repo under the specified node.
If module is provided, also adds the repo to that module in modules.yaml.

Use hv_repo_list with target="nodes" first to discover valid node paths.`,
    {
      deck: z.string().describe("Deck name (e.g. 'cloud-manager'). Use hv_decks to list available decks."),
      repo: z.string().describe("Repo in org/repo format matching a known org in config.yaml (e.g. 'talkersoft-com/my-service')."),
      node: z.string().describe(
        "Node path within the deck tree where the repo will live (e.g. 'tools', 'vm-infra/cloud-manager'). " +
        "Use '' for the deck root. Use hv_repo_list with target='nodes' to see valid options."
      ),
      module: z.string().optional().describe(
        "Optional module name in modules.yaml to also add the repo to. " +
        "Use hv_repo_list with target='modules' to see existing modules."
      ),
      branch: z.string().optional().describe("Branch to push to. Defaults to the deck's configured branch or 'main'."),
      message: z.string().optional().describe("Initial commit message when pushing existing code. Defaults to 'initial commit'."),
    },
    async ({ deck, repo, node, module: mod, branch, message }) => {
      const payload: Record<string, string> = { op: "repo_add", deck, repo, node };
      if (mod) payload.module = mod;
      if (branch) payload.branch = branch;
      if (message) payload.message = message;

      const { ok: success, output } = runHv(payload);
      if (!success) {
        return err(output, "repo_add failed. Check that the org is defined in config.yaml and you have GitHub access.");
      }
      return ok(output, "Deck YAML updated. Run hv_status to verify the repo is visible in the deck.");
    }
  );
}
