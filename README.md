<p align="center">
  <img src="assets/buzzinism.png" alt="Hive Deck" width="400" />
  <br/>
  <h1 align="center">Hive Deck</h1>
  <h3 align="center"><em>"Deterministic multi-repo orchestration for agentic AI workflows."</em></h3>
  <p align="center">Manage hundreds of repos as one. MCP, CLI, or Go — pick your interface.</p>
</p>

---

<p align="center">
  🔒 <strong>Enforced rules</strong> — no partial states, no forgotten repos, no surprises<br/>
  🛠️ <strong>Scoped MCP management</strong> — manages per-deck and per-workspace MCP configurations automatically at init<br/>
  🌿 <strong>Automatic branch management</strong> — deck is always on a named feature branch; main is never checked out locally<br/>
  📂 <strong>VS Code workspace</strong> — a <code>.code-workspace</code> file is automatically generated and kept in sync for every deck
</p>

---

<table align="center" width="100%">
<tr>
<td align="center" width="50%">
<h2>🤖 MCP</h2>
<h4>Talk to your AI agent<br/>it handles every repo</h4>
</td>
<td align="center" width="50%">
<h2>⚡ CLI</h2>
<h4>Full terminal control for scripting and CI</h4>
</td>
</tr>
</table>

<h3 align="center">Unified Features</h3>

<table width="100%">
<tr>
<td>🚀 <strong>Provision + branch</strong></td><td><small>Clone every repo, create a feature branch, and apply MCP config in one shot</small></td>
<td>📦 <strong>Ship</strong></td><td><small>Commit, push, and open PRs for every changed repo simultaneously</small></td>
</tr>
<tr>
<td>🔀 <strong>Next</strong></td><td><small>Transition to a new branch from <code>origin/main</code> — local main never touched</small></td>
<td>🧯 <strong>Stash / unstash</strong></td><td><small>Deadlock escape — stash dirty changes on a merged branch, restore after next</small></td>
</tr>
<tr>
<td>🔄 <strong>Sync</strong></td><td><small>Verify every repo is clean, then pull the whole deck at once</small></td>
<td>📊 <strong>Status</strong></td><td><small>Snapshot the git state of every repo in a single view</small></td>
</tr>
<tr>
<td>📋 <strong>List repos</strong></td><td><small>See every declared repo and whether it's provisioned on disk</small></td>
<td>🔍 <strong>List pulls</strong></td><td><small>Surface all open pull requests across every repo in the deck</small></td>
</tr>
<tr>
<td>🧹 <strong>Teardown</strong></td><td><small>Surgically remove tracked files and <code>.git/</code> — untracked files preserved</small></td>
<td>✂️ <strong>Prune</strong></td><td><small>Remove on-disk repos no longer declared in the deck</small></td>
</tr>
<tr>
<td>➕ <strong>Repo add</strong></td><td><small>Add a new repo to a deck — creates on GitHub, clones, and wires up automatically</small></td>
<td>⏳ <strong>Await merge</strong></td><td><small>Poll until all open PRs on the deck are merged, then auto-transition</small></td>
</tr>
<tr>
<td>📋 <strong>Workflows</strong></td><td><small>Structured plan → orchestrate → execute loop with file-based task scaffolding</small></td>
<td></td><td></td>
</tr>
</table>

---

## What it does

Give an AI agent a deck file and a single command — it provisions an entire multi-repo workspace, creates any missing GitHub repos, branches off main, does the work, ships it, and tears down cleanly. Every step is enforced: you cannot ship from main, you cannot teardown with uncommitted changes, you cannot branch a dirty workspace.

---

> **"Add dark mode to the dashboard — make sure it's consistent across all services."**

The AI provisions all 20 repos onto a fresh feature branch, makes the changes where they're needed, then commits, pushes, and opens a PR for every repo that changed. You typed one sentence. Every repo moved in lockstep — same branch, same rules, nothing left behind.

---

## MCP Setup

### Prerequisites

- Node.js 18+
- Go 1.22+
- `git`
- `gh` CLI (GitHub repo creation + PRs)
- `rsync` (pre-installed on macOS)

### Build and install from source

```sh
# 1. Build and install the hv binary
make install          # binary → $GOBIN, configs → $HOME/.hv/
export PATH="$HOME/go/bin:$PATH"

# 2. Build the MCP server
cd mcp
npm install
npm run build
```

### Configure Claude Code

Add the MCP server to `~/.claude.json`:

```json
{
  "mcpServers": {
    "hive-deck-mcp": {
      "command": "node",
      "args": ["/path/to/hive-deck/mcp/dist/index.js"],
      "env": {
        "MCP_PROFILE": "workflows"
      }
    }
  }
}
```

Restart Claude Code. The `hv_*` tools will appear automatically.

### Profiles

Control which tools are exposed via `MCP_PROFILE`:

| Profile | Tools |
|---|---|
| `decks` | All deck tools: status, list, decks, init, ship, sync, next, stash, unstash, list-pulls, teardown, prune, mcp, await-merge, repo-list, repo-add |
| `workflows` | Everything in `decks` + workflow tools: hv_plan, hv_promote, hv_orchestrate_create, hv_orchestrate_list, hv_orchestrate_run |

---

## Branch naming

The AI **always** picks the branch name — never ask the user for one. Names should be short, whimsical, and have no relation to the task: `velvet-thunder`, `cosmic-hamster`, `silver-mango`. Do not use task-themed names like `ansible-shenanigans` or `auth-overhaul` — the name is just a handle, not a description.

### Full agentic workflow reference

The deck is **always on a named feature branch** — main is never checked out locally.

```
hv_decks          → "my-saas"

hv_init           deck: "my-saas"
                  branch: "ratelimit-reckoning"

  → 18 repos provisioned, all on branch ratelimit-reckoning

[ agent adds rate limiting to auth-service, billing-service, api-gateway ]

hv_ship           deck: "my-saas"
                  message: "feat: add rate limiting"
                  title: "feat: add rate limiting across services"

  → 3 repos committed and pushed
  → 3 pull requests opened and merged (auto_merge: true)
  → deck stays on ratelimit-reckoning (local main never touched)

hv_next           deck: "my-saas"
                  branch: "payment-chaos-rodeo"

  → fetches origin/main into every repo
  → creates payment-chaos-rodeo from origin/main
  → all repos on new branch, ready for next task

[ repeat: work → ship → next ]

hv_teardown       deck: "my-saas"
  → workspace removed; success confirms everything is committed and pushed
```

---

## Workflows

Workflows are a structured plan → orchestrate → execute loop. Instead of giving the AI a blank prompt, workflows produce a file-based scaffold — a PLAN.md, an ORCH.md, and numbered task files — that the agent works through step by step. Every task is written before it runs, every result is written before the next task starts.

### The three-step loop

```
hv_plan           deck: "my-saas"
                  requirements: "Add webhook support to the billing service..."

  → assembles planning scaffold, agent writes PLAN.md
  → PLAN.md is saved to planning/workflow-plans/<deck>/<branch>/PLAN.md

hv_promote        deck: "my-saas"
                  name: "my-saas-feature"
                  plan_paths: ["/path/to/PLAN.md"]

  → assembles orchestration scaffold from the plan
  → agent writes ORCH.md + task files to planning/workflow-exec/<deck>/<branch>/

/loop             → user runs this to begin execution
                  → agent works through tasks one by one
                  → each task: write result → write test → ship → next
```

### Workflow fragments

Workflow behaviour is controlled by **fragment files** — markdown snippets assembled into the prompt at planning and orchestration time. The built-in fragments live in the binary; you can override or extend them from `.hv/workflows/`.

```
.hv/workflows/
  key-rules.md            # enforced rules for every workflow run
  status-check.md         # task 0000 — always hv_status + hv_init/hv_next
  folder-structure.md     # how workflow-exec and workflow-plans are laid out
  write-init.md           # how to write ORCH.md and task 0000
  write-task.md           # how to write a task file
  write-test.md           # how to write a test file
  write-result-lessons.md # how to write results and retro
  write-deck.md           # how to write deck.md
  write-orch.md           # how to write ORCH.md
  write-fix.md            # how to write a fix file after a failed test
  ship-plans.md           # shipping rules
```

### workflows.yaml

Map deck names to workflow extensions and fragment lists. Copy `.hv/workflows.yaml.example` to `.hv/workflows.yaml` and edit:

```yaml
my-saas:
  repo: your-org/workflow-configuration   # where extension .yaml files live
  steps:
    - workflows/status-check.md
    - workflows/folder-structure.md
    - workflows/write-init.md
    - workflows/write-task.md
    - workflows/write-test.md
    - workflows/write-result-lessons.md
    - workflows/ship-plans.md
    - workflows/key-rules.md
```

### Key rules (enforced by the workflow scaffold)

- Results and retro are written **before** `hv_ship` — never after
- Fix files are written **before** retrying a failed test — never silently re-run
- Task 0000 is always `hv_status` + `hv_init`/`hv_next` — never skip it
- All deck operations use MCP tools (`hv_status`, `hv_init`, `hv_next`, `hv_ship`) — never raw `git`

---

## Concepts

- **deck** — a named workspace folder containing a tree of repos
- **node** — a folder inside the deck tree; nodes can hold repos, modules, symlinks, or nested nodes
- **module** — a named bundle of repos defined once in `modules.yaml` and referenced by any deck

On disk: `<decks_root>/<deck>/<node>/.../<repo>/`

---

## Configuration

### Layout

```
.hv/                          # config dir — gitignored except examples and workflow fragments
.hv/config.yaml               # per-machine: decks_root, orgs, branch defaults
.hv/modules.yaml              # named bundles of repos shared across decks
.hv/<name>.yaml               # one deck file per workspace (e.g. my-saas.yaml)
.hv/workflows.yaml            # workflow definitions per deck (gitignored — see example)
.hv/workflows/                # fragment overrides (committed — generic markdown)
.hv/config.yaml.example       # committed template
.hv/mcps.yaml.example         # committed template
.hv/workflows.yaml.example    # committed template
```

**Hive Deck never writes to any YAML file.** All YAML is read-only input.

### Config file lookup — CWD first

1. `$HV_HOME/.hv/<file>` — explicit override; useful in CI
2. `<CWD>/.hv/<file>` — project-local config wins when present
3. `$HOME/.hv/<file>` — global user install fallback

### config.yaml

```yaml
decks_root: ~/workspace

orgs:
  myorg:
    url: github.com/my-github-org
    protocol: https
  personal:
    url: github.com/my-github-username
    protocol: ssh

default_branch: main

branches:
  some-repo: develop        # per-repo branch overrides
```

### modules.yaml

```yaml
platform:
  org: myorg
  repos: [auth-service, billing-service, api-gateway]

shared-libs:
  org: myorg
  repos: [common-lib, utils]
```

### Deck files

```yaml
deck:
  repos:
    - myorg/infra-core
  modules: [shared-libs]
  symlinks:
    - ~/.hv
  workspace_folder: true

  services:
    modules: [platform]
  tools:
    repos:
      - myorg/deploy-cli
      - personal/dotfiles
  nested:
    deeper:
      repos: [myorg/data-pipeline]
```

The deck name comes from the filename stem: `my-saas.yaml` → deck folder `my-saas`.

---

## CLI Reference

> The `hv` CLI is the engine behind the MCP tools. Use it directly for scripting or CI. For agentic workflows, prefer the MCP interface.

### gh setup for headless / CI environments

```sh
export GH_TOKEN=<your-github-pat>
gh auth login --with-token <<< "$GH_TOKEN"
```

---

## Development

```sh
make build      # ./bin/hv
make run ARGS="decks"
make test
make lint
make help
```

```sh
HV_HOME=$PWD ./bin/hv decks    # run without affecting installed configs
```
