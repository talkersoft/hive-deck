import { readFileSync } from "fs";
import { dirname, resolve } from "path";
import { fileURLToPath } from "url";

const __dirname = dirname(fileURLToPath(import.meta.url));

interface ProfileDefinition {
  extends?: string;
  tools: string[];
}

interface ProfilesFile {
  profiles: Record<string, ProfileDefinition>;
}

function loadProfilesFile(): ProfilesFile {
  const path = resolve(__dirname, "../profiles.json");
  return JSON.parse(readFileSync(path, "utf-8")) as ProfilesFile;
}

export function resolveProfile(
  name: string,
  profiles: Record<string, ProfileDefinition> = loadProfilesFile().profiles,
  visited: Set<string> = new Set()
): string[] {
  if (visited.has(name)) {
    throw new Error(`Circular profile extends: ${[...visited, name].join(" -> ")}`);
  }
  const profile = profiles[name];
  if (!profile) {
    throw new Error(`Profile "${name}" not found in profiles.json`);
  }
  visited.add(name);
  const base = profile.extends ? resolveProfile(profile.extends, profiles, visited) : [];
  const merged = [...base];
  for (const tool of profile.tools) {
    if (!merged.includes(tool)) merged.push(tool);
  }
  return merged;
}

export function activeProfileName(): string {
  const argIndex = process.argv.indexOf("--profile");
  if (argIndex !== -1 && process.argv[argIndex + 1]) {
    return process.argv[argIndex + 1];
  }
  return process.env.MCP_PROFILE ?? "workflows";
}
