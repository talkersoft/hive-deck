import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_orchestrate_plan",
    `Assemble the planning orchestration for a deck and return it with the provided requirements.
Calls \`hv orchestrate <deck> planning\` to produce the planning scaffold, then appends the
requirements so the agent has everything it needs to write a PLAN.md in one shot.
IMPORTANT: Do not run this tool until the user has described what they want to build. The requirements field is mandatory.`,
    {
      deck: z.string().describe(
        "Deck name (e.g. 'cloud-manager'). Use hv_decks to list available decks."
      ),
      requirements: z.string().describe(
        "What needs to be built — the system, the change, and the goal. " +
        "Before passing this, clean up the user's language: turn vague phrases into " +
        "precise technical requirements (what entity, what operation, which layer, what the expected outcome is). " +
        "This field is required — do not call this tool without a clear, specific requirements string."
      ),
      name: z.string().optional().describe(
        "Optional workflow name to use instead of the default 'planning' workflow."
      ),
    },
    async ({ deck, requirements, name }) => {
      if (!requirements.trim()) {
        return err(
          "hv_orchestrate_plan requires requirements.\n" +
          "Ask the user to describe what they want to build before calling hv_orchestrate_plan."
        );
      }

      const status = runHv({ op: "status", deck });
      if (!status.ok) {
        return err(status.output, "hv status failed — check that the deck name is correct.");
      }
      if (status.output.includes("DIRTY")) {
        return err(
          status.output,
          "One or more repos are dirty. Clean them up (commit, push, or stash) before starting a new workflow run."
        );
      }

      const workflowName = name?.trim() || "planning";
      const { ok: success, output } = runHv({ op: "orchestrate_plan", deck, name: workflowName });
      if (!success) {
        return err(output);
      }

      const combined = `${output}\n\n## Requirements\n\n${requirements.trim()}`;
      return ok(combined);
    }
  );
}
