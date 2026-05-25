<p align="center">
  <img src="assets/buzzinism.png" alt="Hive Deck" width="400" />
  <br/>
  <h1 align="center">Hive Deck</h1>
</p>

Give an AI agent a deck file and a single command — it provisions the entire multi-repo folder structure your project depends on, creates any repos that don't exist yet, and drops a VS Code workspace ready to open. No interactive steps, no manual cloning, no configuration drift between machines.

`hv` is a small Go CLI that provisions and tears down on-disk developer workspaces from a YAML deck file. It's designed automation-first: drop a `.hv/` directory alongside any payload (a repo, a CI job, an agent working directory) and `hv` will use it without touching your global config.

Teardown is a first-class signal in agentic pipelines. `hv teardown` refuses to run if any repo has uncommitted changes, unpushed commits, or unresolved state — so a successful teardown means the agent finished its work and pushed everything. A failed teardown means it didn't. Pair with the next provision to restore the workspace from scratch: provision → agent works → teardown as completion check → reprovision for the next run.

## Prerequisites

- Go 1.22+
- `git`
- `gh` CLI — used to create missing GitHub repos during provision and to open pull requests
- `rsync` — used to restore torn-down repo shells in place (pre-installed on macOS; may need installing on minimal Linux environments)

### Headless gh setup (CI / agentic pipelines)

```sh
export GH_TOKEN=<your-github-pat>
gh auth login --with-token <<< "$GH_TOKEN"
gh auth status
```

For long-running agents or CI environments, set `GH_TOKEN` in the environment and `gh` will pick it up automatically without an explicit `auth login` step.

## Concepts

- **deck** — a named workspace folder containing a tree of repos
- **node** — a folder inside the deck tree; nodes can hold repos, modules, symlinks, or nested nodes
- **module** — a named bundle of repos defined once in `modules.yaml` and referenced by any deck

On disk: `<decks_root>/<deck>/<node>/.../<repo>/`

## Layout

```
.hv/                          # config dir — gitignored except *.example
.hv/config.yaml               # per-machine: decks_root, orgs, branch defaults
.hv/modules.yaml              # named bundles of repos shared across decks
.hv/<name>.yaml               # one deck file per workspace (e.g. cloud-manager.yaml)
.hv/config.yaml.example       # committed template for config.yaml
```

All hv config lives in `.hv/`. The whole directory is gitignored except for `*.example` templates, so you can author personal deck files without committing them.

**hv never writes to any YAML file.** All YAML is read-only input.

## Config file lookup

Every config file (`config.yaml`, `modules.yaml`, and deck files) is resolved using this search order — **CWD first**:

1. `$HV_HOME/.hv/<file>` — explicit override; useful in CI or scripted workflows
2. `<CWD>/.hv/<file>` — project-local config wins when present
3. `$HOME/.hv/<file>` — global user install fallback

`hv` always checks the current working directory first. In an automated pipeline, point the agent's working directory at any checkout that contains a `.hv/` and it will use those files automatically — no global config required, no environment variables to set.

## Install

### User install (recommended)

```sh
make install
```

Builds the binary and copies `.hv/` to `$HOME/.hv/`. The binary lands in `$GOBIN` (default `~/go/bin`) — make sure that's on `$PATH`:

```sh
export PATH="$HOME/go/bin:$PATH"   # add to ~/.zshrc or ~/.bashrc if missing
```

### System install

```sh
make install-system     # sudo install to /usr/local/bin; configs go to $HOME/.hv/ (no sudo)
make uninstall-system   # remove /usr/local/bin/hv only; does not touch $HOME/.hv/
```

### Configs only

```sh
make install-config     # copy .hv/ -> $HOME/.hv/ without rebuilding the binary
```

Re-running any install target unconditionally overwrites files at the destination. Keep the source of truth in this repo's `.hv/`.

## Quick start

```sh
# 1. Bootstrap config.yaml from the committed example
make setup
$EDITOR .hv/config.yaml           # set decks_root, orgs, default_branch

# 2. Install (binary + configs to $HOME/.hv/)
make install

# 3. Provision a deck
hv provision cloud-manager
```

## config.yaml

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

See [.hv/config.yaml.example](.hv/config.yaml.example) for the full annotated shape including `claude_settings`, `gitignore`, and `readme` options.

## modules.yaml

Defines named bundles of repos. Any deck can reference a module by name.

```yaml
cloud-manager:
  org: myorg
  repos: [cloud-manager-api, cloud-manager-web]

shared-libs:
  org: myorg
  repos: [auth-lib, utils]
```

## Deck files

A deck file describes the folder tree to provision. The top-level key is always `deck:`. Every other key is a folder name; reserved keys on any node are `repos`, `modules`, `symlinks`, and `workspace_folder`.

```yaml
deck:
  repos:
    - myorg/top-level-repo             # clones directly into the deck folder
  modules: [shared-libs]              # expands module repos into the deck folder
  symlinks:
    - ~/.hv                            # symlink created inside the deck folder
  workspace_folder: true              # include deck folder in VS Code workspace

  services:
    modules: [cloud-manager]
  tools:
    repos:
      - myorg/cli-tool
      - personal/dotfiles
  nested:
    deeper:
      repos: [myorg/some-repo]        # nodes nest to any depth
```

The deck name comes from the filename stem: `cloud-manager.yaml` → deck folder `cloud-manager`.

A VS Code `.code-workspace` file is generated at `<decks_root>/<deck>/hive-workspace/<deckname>.code-workspace` after every provision or teardown. The `hive-workspace/` folder is never removed by hv.

## Commands

### `hv provision <deck>`

Clone every declared repo in the deck. Idempotent — safe to run any number of times.

| Repo state on disk | Action |
|---|---|
| Absent | Clone from GitHub |
| Directory present, no `.git/` (shell left by teardown) | Restore in place: clone into temp, rsync non-conflicting files back |
| Already cloned | Skip |

GitHub create-if-missing is always on: for any repo that doesn't yet exist on github.com, `gh repo create --private --add-readme` runs before the clone.

```sh
hv provision cloud-manager
```

**When to use:** starting a new machine, onboarding to a project, or restoring a workspace after teardown.

---

### `hv teardown <deck>`

Surgically remove tracked files and `.git/` from every in-scope repo, leaving all untracked files in place. Prunes now-empty directories.

Refuses to run if any repo has:
- Uncommitted changes
- Unpushed commits
- Detached HEAD
- Stash entries

There is no `--force` or nuke mode. Use `rm -rf` for destructive wipes.

```sh
hv teardown cloud-manager
```

**When to use:** freeing disk space while preserving untracked work, or as a completion signal in agentic pipelines — a successful teardown guarantees all work is committed and pushed.

---

### `hv sync <deck>`

Verify every in-scope repo is fully clean (committed and pushed), then `git pull` each one. Aborts before touching anything if any repo is dirty.

```sh
hv sync cloud-manager
```

**When to use:** pulling in upstream changes at the start of a session after you know your local work is committed and pushed.

---

### `hv status <deck>`

Report the git state of every repo in the deck. Shows clean/dirty status and the specific reason for each dirty repo.

```sh
hv status cloud-manager
```

Example output:
```
=== tools ===
  clean    cloud-manager/tools/hive-deck-pro
  DIRTY    cloud-manager/tools/mcp-manager
             - working tree not clean (uncommitted or untracked changes)
  clean    cloud-manager/tools/database-toolkit

total: 2/3 clean across 1 node(s)
```

**When to use:** before any action command to understand current workspace state, or to diagnose why teardown/sync/pr refused to run.

---

### `hv list <deck>`

List every repo declared by the deck alongside its provisioned state (yes/no).

```sh
hv list cloud-manager
```

Example output:
```
DEST                                     MODULE                    PROVISIONED
tools/hive-deck-pro                      myorg/hive-deck-pro       yes
tools/database-toolkit                   myorg/database-toolkit    yes
vm-infra/cloud-manager/cloud-manager-api myorg/cloud-manager-api   no
```

**When to use:** auditing what a deck declares and which repos are currently on disk.

---

### `hv decks`

List every deck file (`*.yaml`) in the active `.hv/` config directory.

```sh
hv decks
```

Example output:
```
cloud-manager
hive
personal-apps
tooling
```

**When to use:** when you don't remember the exact deck name to pass to other commands.

---

### `hv pr <deck> --title <title> [--body <body>]`

Open a pull request for every repo in the deck that is on a feature branch with commits ahead of `origin/<default>`. The same title and body are applied to every PR created.

Pre-flight checks run across all repos before any PR is created:

| Repo state | Action |
|---|---|
| Dirty working tree | Abort — fix before opening PRs |
| On default branch with unpushed commits | Abort — move work to a feature branch first |
| On default branch, nothing ahead | Skip |
| On feature branch, ahead of `origin/<default>` | Create PR |
| On feature branch, nothing new | Skip |
| PR already open for this branch | Skip |

```sh
hv pr cloud-manager --title "feat: add webhook support"
hv pr cloud-manager --title "fix: auth timeout" --body "Fixes the session expiry bug"
```

All created PR URLs are printed at the end of the run.

**When to use:** after finishing a feature that spans multiple repos — one command opens all the PRs with a consistent title.

---

### `hv default <deck>`

Switch every repo in the deck to its default branch (`main`/`master`) and pull. Verifies all repos are fully clean and pushed before switching anything — if any repo is dirty the command aborts without touching anything.

Safe to run when repos are already on the default branch (they will just be pulled).

```sh
hv default cloud-manager
```

**When to use:** after PRs are merged — pull the merged changes and clear all feature branches in one step, leaving the workspace ready for the next task.

---

### `hv prune <deck> [--dry-run]`

Find every git repo on disk under the deck directory that is not declared in the deck YAML, verify each one is clean, then remove them. Aborts if any undeclared repo is dirty.

```sh
hv prune cloud-manager             # remove all undeclared repos (safe — aborts if any are dirty)
hv prune cloud-manager --dry-run   # preview what would be removed without removing anything
```

**When to use:** after removing repos from a deck file — cleans up the on-disk directories that are no longer declared.

---

## Typical agentic workflow

```sh
hv provision cloud-manager     # restore workspace from deck
# ... agent does work across repos ...
hv status cloud-manager        # verify work is committed and pushed
hv pr cloud-manager --title "feat: agent task name"
hv default cloud-manager       # merge and reset all repos to main
hv teardown cloud-manager      # clean up; success confirms all work is safe
```

## Development

```sh
make build      # ./bin/hv
make run ARGS="decks"
make test
make lint       # fmt + vet (+ golangci-lint if installed)
make help       # list all targets
```

To iterate without affecting your installed configs, run the local binary with `$HV_HOME` pointing at this checkout:

```sh
HV_HOME=$PWD ./bin/hv decks
```
