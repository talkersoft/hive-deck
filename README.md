<p align="center">
  <img src="assets/buzzinism.png" alt="Hive Deck" width="400" />
  <br/>
  <h1 align="center">Hive Deck</h1>
</p>

Give an AI agent a deck file and a single command — it provisions the entire multi-repo folder structure your app depends on, creates any repos that don't exist yet, and drops a VS Code workspace ready to open. No interactive steps, no manual cloning, no configuration drift between machines.

`hv` is a small Go CLI that provisions and tears down on-disk developer workspaces from a YAML deck file. It's designed automation-first: drop a `.hive/` directory alongside any payload (a repo, a CI job, an agent working directory) and `hv` will use it without touching your global config. Deck files are the only input — `hv` never writes to YAML.

Teardown is a first-class signal in agentic pipelines. `hv deck teardown` refuses to run if any repo has uncommitted changes, unpushed commits, or unresolved state — so a successful teardown means the agent finished its work and pushed everything. A failed teardown means it didn't. Pair with the next provision to restore the workspace from scratch: provision → agent works → teardown as completion check → reprovision for the next run.

## Prerequisites

- Go 1.22+
- `git`
- `gh` CLI — used to create missing GitHub repos during provision
- `rsync` — used to restore torn-down repo shells in place (pre-installed on macOS; may need installing on minimal Linux environments)

### Headless gh setup (CI / agentic pipelines)

```sh
# authenticate with a token — no browser, no prompts
export GH_TOKEN=<your-github-pat>
gh auth login --with-token <<< "$GH_TOKEN"

# or pass the token inline without storing it
echo "$GH_TOKEN" | gh auth login --with-token

# verify
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
.hive/                        # config dir — gitignored except *.example
.hive/setup.yaml              # per-machine: decks_root, orgs, branch defaults
.hive/modules.yaml            # named bundles of repos shared across decks
.hive/<name>.yaml             # one deck file per workspace (e.g. cloud-manager.yaml)
.hive/setup.yaml.example      # committed template for setup.yaml
```

All hv config lives in `.hive/`. The whole directory is gitignored except for `*.example` templates, so you can author personal deck files without committing them.

**hv never writes to any YAML file.** All YAML is read-only input.

## Config file lookup

Every config file (`setup.yaml`, `modules.yaml`, and deck files) is resolved using this search order — **CWD first**:

1. `$HV_HOME/.hive/<file>` — explicit override; useful in CI or scripted workflows
2. `<CWD>/.hive/<file>` — project-local config wins when present
3. `$HOME/.hive/<file>` — global user install fallback

`hv` always checks the current working directory first. In an automated pipeline, point the agent's working directory at any checkout that contains a `.hive/` and it will use those files automatically — no global config required, no environment variables to set.

## Install

### User install (recommended)

```sh
make install
```

Builds the binary and copies `.hive/` to `$HOME/.hive/`. The binary lands in `$GOBIN` (default `~/go/bin`) — make sure that's on `$PATH`:

```sh
export PATH="$HOME/go/bin:$PATH"   # add to ~/.zshrc or ~/.bashrc if missing
```

### System install

```sh
make install-system     # sudo install to /usr/local/bin; configs go to $HOME/.hive/ (no sudo)
make uninstall-system   # remove /usr/local/bin/hv only; does not touch $HOME/.hive/
```

### Configs only

```sh
make install-config     # copy .hive/ -> $HOME/.hive/ without rebuilding the binary
```

Re-running any install target unconditionally overwrites files at the destination. Keep the source of truth in this repo's `.hive/`.

## Quick start

```sh
# 1. Bootstrap setup.yaml from the committed example
make setup
$EDITOR .hive/setup.yaml          # set decks_root, orgs, default_branch

# 2. Install (binary + configs to $HOME/.hive/)
make install

# 3. Provision a deck
hv deck provision cloud-manager
```

## setup.yaml

```yaml
decks_root: ~/hive

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

See [.hive/setup.yaml.example](.hive/setup.yaml.example) for the full annotated shape including `claude_settings`, `gitignore`, and `readme` options.

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
deck:                                  # root node — all keys work here, not just in children
  repos:
    - myorg/top-level-repo             # clones directly into the deck folder
  modules: [shared-libs]              # expands module repos into the deck folder
  symlinks:
    - ~/.hive                          # symlink created inside the deck folder
  workspace_folder: true              # include deck folder in VS Code workspace

  config:                              # just a folder name — no special meaning
    symlinks:
      - ~/.hive
    workspace_folder: true
  services:
    modules: [cloud-manager]
  tools:
    repos:
      - myorg/cli-tool                 # org-key/repo — org-key matches setup.yaml orgs:
      - personal/dotfiles
  nested:
    deeper:
      repos: [myorg/some-repo]        # nodes nest to any depth
```

The deck name comes from the filename stem: `cloud-manager.yaml` → deck folder `cloud-manager`.

A VS Code `.code-workspace` file is generated at `<decks_root>/<deck>/hive-workspace/<deckname>.code-workspace` after every provision or teardown. The `hive-workspace/` folder is never removed by hv.

## Usage

```sh
hv deck provision <deck>                       # idempotent: clone missing, restore shells, skip existing
hv deck provision <deck> --filter <node>       # provision only the subtree rooted at <node>
hv deck teardown  <deck> [--filter <node>]     # remove tracked files + .git/; preserve untracked
hv deck status    <deck> [--filter <node>]     # report git state across in-scope repos
hv deck list      <deck>                       # list repos with provisioned state
hv deck decks                                  # list all deck files in the active .hive/
```

### provision

Idempotent — safe to run any number of times. For each declared repo:

- absent → clone
- dir present, `.git/` absent (shell left by teardown) → restore in place: clone into temp, then `rsync -a --ignore-existing` so preserved untracked files stay put
- dir present, `.git/` present → skip

GitHub create-if-missing is always on: for each repo that doesn't yet exist on github.com, `gh repo create --private --add-readme` runs before the clone.

### teardown

Surgical preserve is the only teardown mode — no `--force`, no nuke. For each in-scope repo:

- Deletes every git-tracked file and the `.git/` folder
- Leaves all untracked files in place (gitignored or not)
- Prunes now-empty directories
- Refuses if any repo has tracked work that would be lost (uncommitted changes, unpushed commits, detached HEAD, stash entries)

Re-running teardown is safe and idempotent.

## Development

```sh
make build      # ./bin/hv
make run ARGS="deck decks"
make test
make lint       # fmt + vet (+ golangci-lint if installed)
make help       # list all targets
```

To iterate without affecting your installed configs, run the local binary with `$HV_HOME` pointing at this checkout:

```sh
HV_HOME=$PWD ./bin/hv deck decks
```
