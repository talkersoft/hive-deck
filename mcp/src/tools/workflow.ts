import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { readFileSync } from "fs";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_plan",
    `Assemble the planning workflow for a deck and return it with the provided requirements.
Calls \`hv workflow <deck> planning\` to produce the planning scaffold, then appends the
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
    },
    async ({ deck, requirements }) => {
      if (!requirements.trim()) {
        return err(
          "hv_plan requires requirements.\n" +
          "Ask the user to describe what they want to build before calling hv_plan."
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

      const { ok: success, output } = runHv({ op: "orchestrate", deck, name: "planning" });
      if (!success) {
        return err(output);
      }

      const combined = `${output}\n\n## Requirements\n\n${requirements.trim()}`;
      return ok(combined);
    }
  );

  server.tool(
    "hv_promote",
    `Promote a reviewed plan into an executable orchestration workflow.
Takes one or more completed PLAN.md files and assembles the orchestration scaffold (ORCH.md, tasks, tests) for execution.
Calls \`hv promote <deck> <name>\` and prepends each plan at the top of the output in order.
Use hv_decks to find valid deck names, and \`hv promote --list <deck>\` to find orchestration names.
IMPORTANT: plan_paths is always required — never call this tool without at least one completed PLAN.md from hv_plan.`,
    {
      deck: z.string().describe(
        "Deck name (e.g. 'cloud-manager'). Use hv_decks to list available decks."
      ),
      name: z.string().describe(
        "Orchestration name (e.g. 'hive-deck-feature'). Run `hv promote --list <deck>` to see available names."
      ),
      plan_paths: z.array(z.string()).min(1).describe(
        "Array of absolute paths to PLAN.md files produced by hv_plan for this work. " +
        "Must be absolute paths (e.g. [\"/Users/you/workspace/cloud-manager/planning/workflow-plans/cloud-manager/thorny-catapult/PLAN.md\"]). " +
        "Plans are prepended in array order, each separated by --- so the agent reads all context before executing tasks. " +
        "At least one path is required."
      ),
    },
    async ({ deck, name, plan_paths }) => {
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

      const planSections: string[] = [];
      for (const planPath of plan_paths) {
        let planContent: string;
        try {
          planContent = readFileSync(planPath, "utf8");
        } catch (e) {
          return err(`Could not read plan file at ${planPath}: ${(e as Error).message}`);
        }
        const label = plan_paths.length > 1
          ? `## Plan ${planSections.length + 1} of ${plan_paths.length}`
          : `## Plan`;
        planSections.push(`${label}\n\n> Source: ${planPath}\n\n${planContent.trim()}`);
      }

      const { ok: success, output } = runHv({ op: "orchestrate", deck, name });
      if (!success) {
        return err(output);
      }

      const combined =
        planSections.join("\n\n---\n\n") +
        `\n\n---\n\n## Orchestration: ${name}\n\n` +
        output;

      return ok(combined);
    }
  );
}
