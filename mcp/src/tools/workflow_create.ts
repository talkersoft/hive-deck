import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { readFileSync } from "fs";
import { runHv, ok, err } from "../runner.js";

export function register(server: McpServer): void {
  server.tool(
    "hv_orchestrate_create",
    `Assemble orchestration scaffold instructions for a deck and return them with plan context.
The agent uses the output to create ORCH.md, task files, and test files in workflow-exec.
Omit name to use the built-in base orchestration only.
Pass name to use a named extension (base + extension merged).
plan_paths should reference PLAN.md files produced by hv_plan or hv_orchestrate_plan.`,
    {
      deck: z.string().describe(
        "Deck name (e.g. 'hive-deck-pro'). Use hv_decks to list available decks."
      ),
      name: z.string().optional().describe(
        "Extension name (e.g. 'hive-deck-feature'). Omit for base scaffold only. Use hv_orchestrate_list to see available names."
      ),
      plan_paths: z.array(z.string()).min(1).describe(
        "Array of absolute paths to PLAN.md files to prepend as context. " +
        "At least one plan is required — never scaffold a workflow without a completed plan."
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
          "One or more repos are dirty. Clean them up before scaffolding a workflow."
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

      const { ok: success, output } = runHv({ op: "orchestrate_create", deck, name: name ?? "" });
      if (!success) {
        return err(output);
      }

      const workflowHeader = name
        ? `## Orchestration: ${name}\n\n`
        : `## Orchestration (base)\n\n`;

      const combined =
        planSections.join("\n\n---\n\n") +
        `\n\n---\n\n` +
        workflowHeader +
        output;

      return ok(combined);
    }
  );
}
