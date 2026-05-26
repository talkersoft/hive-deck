<p align="center">
  <img src="assets/buzzinism.png" alt="Hive Deck" width="400" />
  <br/>
  <h1 align="center">Hive Deck</h1>
  <h3 align="center"><em>"Deterministic multi-repo orchestration for agentic AI."</em></h3>
</p>

---

<table align="center" width="100%">
<tr>
<td align="center" width="50%">
<h2>🤖 MCP</h2>
<h4>Talk to your AI agent — it handles every repo</h4>

| `hv_init` | `hv_ship` | `hv_next` | `hv_stash` / `hv_unstash` |
|---|---|---|---|
| `hv_sync` | `hv_status` | `hv_list` | `hv_list_pulls` |
| `hv_decks` | `hv_teardown` | `hv_prune` | `hv_mcp` |

</td>
<td align="center" width="50%">
<h2>⚡ CLI</h2>
<h4>Full terminal control for scripting and CI</h4>

| `hv init` | `hv ship` | `hv next` | `hv stash push` / `pop` |
|---|---|---|---|
| `hv sync` | `hv status` | `hv list repos` | `hv list pulls` |
| `hv list decks` | `hv teardown` | `hv prune` | `hv mcp` |

</td>
</tr>
</table>

<br/>

| | | | |
|---|---|---|---|
| 🚀 **Provision + branch** | Clone every repo, create a feature branch, and apply MCP config in one shot | 📦 **Ship** | Commit, push, and open PRs for every changed repo simultaneously |
| 🔀 **Next** | Transition to a new branch from `origin/main` — local main never touched | 🧯 **Stash / unstash** | Deadlock escape — stash dirty changes on a merged branch, restore after next |
| 🔄 **Sync** | Verify every repo is clean, then pull the whole deck at once | 📊 **Status** | Snapshot the git state of every repo in a single view |
| 📋 **List repos** | See every declared repo and whether it's provisioned on disk | 🔍 **List pulls** | Surface all open pull requests across every repo in the deck |
| 🧹 **Teardown** | Surgically remove tracked files and `.git/` — untracked files preserved | ✂️ **Prune** | Remove on-disk repos no longer declared in the deck |

<p align="center">
  🔒 <strong>Enforced rules</strong> — no partial states, no forgotten repos, no surprises<br/>
  🛠️ <strong>Auto-configured</strong> — MCP servers and .gitignore rules applied to every repo at init<br/>
  🌿 <strong>Automatic branch management</strong> — deck is always on a named feature branch; main is never checked out locally
</p>

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
      "args": ["/path/to/hive-deck-pro/mcp/dist/index.js"],
      "env": {
        "MCP_PROFILE": "hv.deck.pro"
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
| `hv.deck` | status, list, decks |
| `hv.deck.operator` | + init, ship, sync, next, stash, unstash, list-pulls |
| `hv.deck.pro` | + teardown, prune, mcp |

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

## Concepts

- **deck** — a named workspace folder containing a tree of repos
- **node** — a folder inside the deck tree; nodes can hold repos, modules, symlinks, or nested nodes
- **module** — a named bundle of repos defined once in `modules.yaml` and referenced by any deck

On disk: `<decks_root>/<deck>/<node>/.../<repo>/`

---

## Configuration

### Layout

```
.hv/                          # config dir — gitignored except *.example
.hv/config.yaml               # per-machine: decks_root, orgs, branch defaults
.hv/modules.yaml              # named bundles of repos shared across decks
.hv/<name>.yaml               # one deck file per workspace (e.g. cloud-manager.yaml)
.hv/config.yaml.example       # committed template
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

> The `hv` CLI is the engine behind the MCP tools. Use it directly for scripting or CI. For agentic workflows, prefer the MCP interface. Full syntax is in the Commands table above.

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
