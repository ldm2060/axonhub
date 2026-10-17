---
name: ci-watch
description: Poll GitHub Actions for this fork (ldm2060/axonhub), diagnose failing runs, fix them locally, and push so CI reruns. Use when asked to watch, poll, monitor, or check CI, when a CI run is failing, or when a cron/loop fires a "check CI" prompt. Covers run lookup by commit, failure-log extraction, reproducing CI-only lint failures locally, the mandatory build/lint/test gate, and the push-and-recheck cycle. Trigger on "CI", "check CI", "CI failed", "watch CI", "监控 CI", "修复 CI", "跑 CI", or a scheduled CI check.
---

# Watch and Fix CI

The fork lives at `ldm2060/axonhub` (remote `origin`). `gh` in this working directory defaults to the upstream repo `looplj/axonhub`, so **always pass `--repo ldm2060/axonhub`** to every `gh run` command. Checking upstream's runs by mistake is the most common failure of this workflow.

## 1. Find the run for the current commit

The branch under watch is `unstable`.

```bash
gh run list --repo ldm2060/axonhub --branch unstable --limit 10
```

Prefer matching the run to the pushed HEAD:

```bash
git rev-parse --short HEAD
gh run list --repo ldm2060/axonhub --branch unstable --limit 5 --json databaseId,headSha,status,conclusion,workflowName,displayTitle
```

Decision rules:

- If the latest runs for HEAD are `queued` or `in_progress`, do nothing. Report "CI still running" and wait for the next poll.
- If every job for HEAD is `success`, report "CI green for <sha>" and stop working.
- If any job is `failure`, go to step 2.
- Do not fix runs whose `headSha` is not the current HEAD — they are stale.

## 2. Extract the failure

```bash
gh run view <run-id> --repo ldm2060/axonhub --log-failed
```

Identify the failing workflow/job and the first real error (not the workflow boilerplate). Workflows are Build, Lint, and Test; frontend checks are folded into Lint.

## 3. Reproduce locally, then fix

Run the same command CI runs, in the module that failed. Per `AGENTS.md`, the full gate is:

```bash
GOEXPERIMENT=jsonv2 go build ./...
cd llm && GOEXPERIMENT=jsonv2 go build ./...
GOEXPERIMENT=jsonv2 golangci-lint run --timeout 10m --max-same-issues 50 ./...
cd llm && GOEXPERIMENT=jsonv2 golangci-lint run --timeout 10m --max-same-issues 50 ./...
GOEXPERIMENT=jsonv2 go test ./...
cd llm && GOEXPERIMENT=jsonv2 go test ./...
```

Frontend (when `frontend/**` changed):

```bash
cd frontend && pnpm format:check && pnpm lint && pnpm typecheck && pnpm test:unit
```

**CI-only lint failures:** GitHub installs golangci-lint `latest`, which can be newer than the local binary. A local "0 issues" is not proof. When CI reports `File is not properly formatted (gci)` at `file:line`:

1. Inspect that exact line. gci commonly flags a leftover run of two blank lines (e.g. after removing conflict markers) or import grouping.
2. Clear the cache before re-running: `golangci-lint cache clean`.
3. Check every file changed since the base: `gofmt -l <changed files>`.
4. Scan for double blank lines in changed Go files — they are the usual residue of conflict resolution.
5. If the mismatch persists, compare versions (`golangci-lint version`) and consider running the newer release; do not lower `--max-warnings` or disable the linter.

Fix root causes. Never bypass with `--no-verify`, never delete or skip a failing test, and never weaken lint configuration to make CI green.

## 4. Verify before pushing

Re-run the commands from step 3 for the affected module. All must pass. Do not push a fix that only compiles.

## 5. Commit and push

- One focused commit per failure, e.g. `fix(ci): ...` or the repository's prevailing style (`fix:`, `style(...)`, `test(...)`).
- End the message with the standard attribution line.
- Push to `origin unstable`. Pushing is authorized by the user's request to watch and fix CI; do not force-push and do not amend commits that are already published.
- Do not push unrelated working-tree changes. Check `git status` first and stage only the fix.

## 6. Recheck

After pushing, the new HEAD starts a fresh run. Report the new run id and the remaining state. The next cron tick re-checks it; do not busy-wait or poll within one turn.

## Boundaries

- Only the `unstable` branch of `ldm2060/axonhub` is in scope.
- Never enable auto-merge, never merge a PR, and never modify upstream (`looplj/axonhub`).
- Do not rewrite history on `unstable`; CI watches pushed commits, so every fix is a new commit.
- If a failure is caused by an upstream regression that also needs an upstream-side decision, report it instead of guessing.
- If the failure log is truncated or missing, re-fetch with `gh run view <run-id> --repo ldm2060/axonhub --log`, or inspect the job directly.
