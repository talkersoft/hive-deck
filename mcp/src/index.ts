#!/usr/bin/env node

{
  const major = parseInt(process.versions.node.split(".")[0], 10);
  if (major < 18) {
    process.stderr.write(
      `[hive-deck-mcp] Node ${process.versions.node} is too old — needs >=18.\n` +
        `  Install via nvm: nvm install --lts && nvm alias default 'lts/*'\n` +
        `  Then restart Claude Code so it picks up the new node on PATH.\n`
    );
    process.exit(1);
  }
}

import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { activeProfileName, resolveProfile } from "./profiles.js";
import * as status from "./tools/status.js";
import * as list from "./tools/list.js";
import * as decks from "./tools/decks.js";
import * as provision from "./tools/provision.js";
import * as sync from "./tools/sync.js";
import * as defaultBranch from "./tools/default.js";
import * as pr from "./tools/pr.js";
import * as teardown from "./tools/teardown.js";

const allTools: Record<string, { register: (server: McpServer) => void }> = {
  status,
  list,
  decks,
  provision,
  sync,
  default: defaultBranch,
  pr,
  teardown,
};

const profileName = activeProfileName();
const enabledTools = resolveProfile(profileName);

console.error(`[hive-deck-mcp] profile="${profileName}" tools=[${enabledTools.join(", ")}]`);

const server = new McpServer({
  name: "Hive Deck",
  version: "0.1.0",
});

for (const toolName of enabledTools) {
  const tool = allTools[toolName];
  if (!tool) {
    console.error(`[hive-deck-mcp] warning: tool "${toolName}" listed in profile but not implemented`);
    continue;
  }
  tool.register(server);
}

const transport = new StdioServerTransport();
await server.connect(transport);
