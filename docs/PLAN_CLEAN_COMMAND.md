# Plan: `hv deck clean` command

## Purpose

A lightweight command that checks whether every repo in a deck is in a clean, fully-pushed state. Exits `0` if clean, `1` if not. Designed as a pipeline gate — run it before teardown, before a deploy, or as a health check in any automated workflow.

## Behaviour

- Iterates every in-scope repo in the deck (respects `--filter`)
- For each repo, runs the same checks teardown uses: uncommitted changes, unpushed commits, no upstream, detached HEAD, stash entries
- If all repos are clean — prints a summary and exits `0`
- If any repo is dirty — prints which repos and why, exits `1`
- Missing repos (not yet provisioned) count as unclean

## Output

**Clean:**
```
clean: cloud-manager (13/13 repos clean)
```

**Dirty:**
```
  [vm-infra/cloud-manager]
    cloud-manager-api
      - uncommitted changes to tracked files
    vorch-service
      - local commits not pushed to upstream
error: 2 repo(s) are not clean
```

## Implementation

- Add `cleanCmd()` in `cmd/hv/main.go` under `hv deck`
- Reuse `git.CheckTracked()` — same function teardown uses
- Reuse `resolve.Build()` for the repo plan
- No new logic needed in `internal/` — wire existing pieces together

## Notes

- `hv deck status` already prints similar info but never exits non-zero — `clean` is the strict, exit-code-driven version for pipelines
- The `--filter` flag should be supported so you can gate on a specific subtree
- No changes to teardown or provision — this is a read-only check
