---
name: merge-upstream
description: Merge changes from upstream (looplj/axonhub) into our fork (ldm2060/axonhub). Use whenever merging, syncing, pulling, rebasing, or integrating upstream changes — including resolving merge conflicts from `git merge remote/unstable`, cherry-picking upstream commits, or synchronizing with the upstream repo. Covers four fork-specific hazards: (1) fields we've locally modified that upstream also touches, (2) upstream adding DB fields/migrations that must be reconciled with our actual schema and release version, (3) upstream reimplementing features we already shipped, (4) post-merge upgrade verification on a real local deploy. Trigger on any mention of "upstream", "merge upstream", "sync upstream", "拉取上游", "合并上游", "resolve conflicts from upstream", or when a merge from `remote/` is in progress or about to be committed.
---

# Merge Upstream Changes

This fork (`ldm2060/axonhub`, remote `origin`) tracks upstream (`looplj/axonhub`, remote `remote`). Upstream merges are routine but hazardous because we have divergent schema, migrations, and features. This skill walks the four hazards in order, then verifies on a real deploy.

## Setup before resolving

Run these first to understand the divergence:

1. `git fetch remote` — refresh upstream refs.
2. `git log --oneline origin/unstable..remote/unstable` — what upstream has that we don't.
3. `git log --oneline remote/unstable..origin/unstable` — what we have that upstream doesn't (our divergent work).
4. Identify the files in conflict: `git diff --name-only --diff-filter=U`.

Do not blindly accept `--theirs` or `--ours` on schema, migration, or scope files. Each conflict in those areas needs a conscious decision below.

## Hazard 1 — Fields we modified that upstream also changed

We have renamed, retyped, or repurposed fields in Ent schemas (`internal/ent/schema/*.go`), GraphQL, and the frontend. Upstream's changes may be written against the *old* field name/shape they hold, not ours.

For each conflicting schema or GraphQL file:

- Diff our version vs upstream's: `git diff HEAD...remote/unstable -- <file>`. Determine whether upstream is reading, writing, or restructuring a field we've already altered.
- If upstream modifies a field we renamed/removed: translate upstream's intent onto our current field. Do not restore the old field name — that breaks our existing data and callers.
- If upstream adds a constraint (index, unique, nullable flip) to a field we changed: check whether the constraint still makes sense on our field. Re-derive it against our schema, don't copy it verbatim.
- Regenerate Ent after schema resolution: `go generate ./internal/ent/...` (or the project's codegen command). The generated code must reflect the merged schema, not a mix of both sides.
- Check GraphQL resolvers (`internal/server/gql/`) and frontend `gql/` for references to the renamed field — upstream's merge may have reintroduced the old name.

**Why:** accepting upstream's version silently reverts our divergent schema changes and corrupts data already stored under our field names. The fix is translation, not selection.

## Hazard 2 — Upstream DB migrations vs our version and schema

Upstream ships schema changes via two mechanisms in this repo:

- **Atlas/SQL migrations** under `internal/ent/migrate/migrations/` (timestamped `.sql` files).
- **Data migrations** under `internal/ent/migrate/datamigrate/` — Go files implementing the `DataMigrator` interface (`Version() string` + `Migrate(ctx, *ent.Client) error`), registered in `NewMigrator` in `internal/ent/migrate/datamigrate/migrator.go`, gated by semver comparison against the system version.

When upstream adds a migration:

- Check whether the migrated field/column already exists in our schema under a different name or type. If so, the migration must be rewritten to target *our* column, or skipped if it's a no-op against our schema.
- **Rename the migration to our version track.** This is the #1 mistake in past merges — do it right:
  1. `git fetch --tags`
  2. `git tag --sort=-v:refname | grep -E '^v0\.2\.' | head -1` → e.g. `v0.2.7`
  3. New version = that tag + 1 patch → `v0.2.8`
  4. Rename the migration file: `v1.0.0-beta6.go` → `v0.2.8.go` (and `_test.go`)
  5. In the `.go` file, replace the struct/func/version string: `V1_0_0_Beta6`/`NewV1_0_0_Beta6`/`"v1.0.0-beta6"` → `V0_2_8`/`NewV0_2_8`/`"v0.2.8"`
  6. In `migrator.go`, replace `NewV1_0_0_Beta6()` → `NewV0_2_8()`
  7. Update `internal/build/VERSION` to the same `v0.2.8`
  8. `git rm` the old upstream-named files, `git add` the new ones

  **Common mistakes to avoid:**
  - ❌ Guessing from existing migration files under `datamigrate/` (they are sparse — `v0.1.10, v0.1.34, v0.1.35` — far behind the real latest release)
  - ❌ Reusing upstream's label verbatim (`v1.0.0-beta6`, etc.) — different version track, will misfire the semver gate
  - ❌ Looking at `internal/build/VERSION` before fixing it — it still holds upstream's label after the merge
  - ❌ Looking at the `v0.1.x` tags — our release track has moved to `v0.2.x`; deriving from the old `v0.1.x` track yields a version that already exists or is far behind, and the semver gate will never fire

- You must reserve upstream commits. DO NOT DELETE THEM.
- Register any new `DataMigrator` in `NewMigrator` in version order.
- If upstream's migration assumes a clean upstream schema (columns we don't have, or columns we added that upstream doesn't know about), adapt the SQL/Go to our actual schema. A migration that `ALTER TABLE`-s a non-existent column will fail at startup.
- Never delete or reorder existing registered migrations — users on older versions still need them. Only append.

**Why:** the migration runs on startup against whatever schema and system version the user's DB holds. The semver gate means a wrongly-tagged version either runs when it shouldn't or never runs when it should. Past incidents: (1) blindly kept upstream's `v1.0.0-beta6`; (2) derived `v0.1.36` from the highest migration filename instead of the latest git tag (`v0.1.62` → should have been `v0.1.63`); (3) derived `v0.1.69` from the old `v0.1.x` tag track when releases had moved to `v0.2.x` (latest `v0.2.7` → should have been `v0.2.8`). All were caught by the user post-commit. The correct procedure is always: **latest `v0.2.x` tag + 1**, nothing else.

## Hazard 3 — Upstream reimplemented a feature we already shipped

We have features upstream doesn't (scope-level RouteGuard, OpenCode Go quota, sponsor-link handling, custom API-key profiles, usage stats, etc. — see the `merge: resolve conflicts with upstream ...` commits in history). Upstream sometimes ships a similar or overlapping feature.

For each such conflict:

- Compare the two implementations on: correctness, completeness, maintainability, and integration with our divergent schema/scopes. Don't prefer ours reflexively — upstream's may be better or may carry fixes we'd otherwise lose.
- Prefer keeping ours when it's wired into our schema/scopes/i18n in ways upstream's isn't (ripping ours out means a cascade of follow-up deletions and a migration to drop the now-unused data).
- Prefer taking upstream's when it's a clean fix to a bug we also have, or a feature we hadn't shipped and that fits our schema without adaptation.
- When taking a hybrid: make the integration explicit — note in the commit message which side each piece came from and why. Hybrids are the most error-prone; the note is for future-you during the next merge.
- Preserve our module path `github.com/ldm2060/axonhub` everywhere (upstream uses `github.com/looplj/axonhub`). Conflicts in `go.mod`, `fx_module.go`, and imports that touch the module path must keep `ldm2060`.

**Why:** these are judgment calls with no mechanical answer. The cost of a wrong call is a regression that survives until the next merge, so prefer the choice that minimizes downstream cascade over the choice that looks cleanest in isolation.

## Hazard 4 — Verify on a real local deploy before declaring done

Do not commit until the merge is verified end-to-end on a running instance, including the upgrade path. Per AGENTS.md, after backend Go changes you must build, restart, and verify in the browser. For a merge, the bar is higher: verify both that the merged build runs, and that an *old-version* DB upgrades cleanly.

Sequence:

1. **Migration version sanity check (do this FIRST, before build/test):** if any migration was added/renamed in Hazard 2, re-run `git tag --sort=-v:refname | grep -E '^v0\.2\.' | head -1` and confirm the migration's `Version()` string, filename, `migrator.go` registration, and `internal/build/VERSION` all equal `latest_tag + 1 patch`. If they don't match, fix before doing anything else — do not build/test/commit a wrongly-versioned migration.
2. `go build ./...` and `cd llm && go build ./...` — both must compile.
3. `golangci-lint run --timeout 10m --max-same-issues 50 ./...` and `cd llm && golangci-lint run ...` — must pass.
4. `go test ./...` and `cd llm && go test ./...` — must pass.
5. `go build -o axonhub.exe ./cmd/axonhub/`, stop the running `axonhub.exe`, start the new one. Watch startup logs for migration execution and errors — the datamigrate `Migrator` runs at boot; a failing migration surfaces here.
6. Open the app in the browser, exercise the affected pages, check for errors (console + network).
7. **Upgrade-path check:** if any Hazard 2 migration was added or modified, verify against the old-version instance kept on the local machine (a real running instance of the previous release with its own DB, not a fresh DB). Point the new binary at that old instance's DB — or start the old instance, confirm it boots, then upgrade it in place to the new binary — and confirm migrations run and the app is functional afterward. This catches the case where migrations are correct on a fresh DB but break on real old data.

If any step fails, fix before committing. A merge that compiles but fails migration on a real DB is worse than no merge — it wedges users on startup.

## Committing the merge

Per [[feedback_verify_and_commit]] and AGENTS.md: run the verification commands, then commit immediately. Use the established commit message style — `merge: resolve conflicts with upstream <short description of what was merged>`. Do not commit `.exe` binaries (see [[no_exe_in_commits]]). After the merge commit lands on the working branch, merge it back to `unstable` per [[feedback_merge_to_unstable]].

**The merge commit MUST be a true merge with two parents.** Upstream's commits (`dba642a0`, etc.) must appear in our history as the second parent of the merge commit, not be squashed into a single-parent "merge:" commit. If they are squashed, the next upstream merge will not see them as ancestors and will re-raise every conflict we already resolved, plus `git log origin/unstable..HEAD` will under-count our divergence and break the merge-setup surveys in this skill.

How to get this right:

- Start the merge with `git merge remote/unstable` (or `git merge <upstream-sha>`) — never by cherry-picking or hand-applying upstream's diff onto a single-parent commit. `git merge` automatically creates the two-parent structure once conflicts are resolved and committed.
- Do NOT use `git merge --squash`. It produces a single-parent commit that looks like a merge but doesn't record the parent link.
- If you discover after the fact that you created a single-parent "merge:" commit (e.g. by `git checkout`-ing files and committing normally), rewrite it before pushing:
  1. `NEW=$(git commit-tree <bad-commit>^{tree} -p <local-parent> -p <upstream-tip> -m "$(git log --format=%B -n 1 <bad-commit>)")`
  2. For each commit on top of `<bad-commit>`, recreate it with `git commit-tree <c>^{tree} -p $NEW -m "..."` and update `$NEW` to the new sha.
  3. `git update-ref refs/heads/<branch> $NEW`
- Verify the result: `git log --graph --oneline -10` should show the upstream commits as a side branch merged in by your merge commit, and `git merge-base HEAD remote/unstable` should equal the upstream tip you merged.
- Before pushing, run the frontend CI steps locally so a missing format/lint/typecheck failure doesn't surface only on the runner: `cd frontend && pnpm lint && pnpm format:check && pnpm typecheck && pnpm test:unit && pnpm build && pnpm bundle:check`. Past incident: a merge commit passed `pnpm lint` and `pnpm typecheck` locally but failed CI at `pnpm format:check` because we hadn't run prettier on the merged files — fix by running `pnpm format` and committing the result as a follow-up `style(frontend): apply prettier formatting to satisfy format:check`.
- You mush retain the upstream commits in the merge commit's history. Do not squash them into a single commit or rebase them onto our branch — that breaks future merges and obscures the provenance of upstream changes.

## When to escalate to the user

Pause and ask before:
- Dropping one of our shipped features entirely in favor of upstream's.
- Deleting or rewriting a registered data migration that has already shipped to users (data-loss risk).
- Renaming a column or field that requires a destructive schema change.
- A conflict where the right call genuinely depends on product intent, not mechanical correctness.
