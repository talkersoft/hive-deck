import { spawnSync } from "child_process";

const HV_BIN = process.env.HV_BIN ?? "hv";

export interface RunResult {
  ok: boolean;
  output: string;
}

export function runHv(payload: object): RunResult {
  const result = spawnSync(HV_BIN, [], {
    input: JSON.stringify(payload),
    encoding: "utf-8",
    stdio: ["pipe", "pipe", "pipe"],
  });

  const stdout = result.stdout ?? "";
  const stderr = result.stderr ?? "";
  const output = (stdout + stderr).trim();
  const ok = result.status === 0;

  if (result.error) {
    return {
      ok: false,
      output: `Failed to run hv: ${result.error.message}\nEnsure the hv binary is installed and on PATH (or set HV_BIN env var).`,
    };
  }

  return { ok, output };
}

export function ok(output: string, hint?: string): ReturnType<typeof result> {
  return result(true, output, hint);
}

export function err(output: string, hint?: string): ReturnType<typeof result> {
  return result(false, output, hint);
}

function result(
  success: boolean,
  output: string,
  hint?: string
): { content: [{ type: "text"; text: string }]; isError?: true } {
  const text = hint ? `${output}\n\n[hint: ${hint}]` : output;
  return success
    ? { content: [{ type: "text" as const, text }] }
    : { content: [{ type: "text" as const, text }], isError: true as const };
}
