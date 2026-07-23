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
- **Derive the new migration version from our latest git tag, NOT from the existing migration file names or from upstream's version label.** This is the single most error-prone step — getting it wrong wedges the semver gate. Do it by running `git fetch --tags` then `git tag --sort=-v:refname | grep -E '^v0\.1\.' | head -1` (filter to our `v0.1.x` series; upstream `v1.0.0-beta*` tags are a different track and must be ignored). The new migration's `Version()` is the **next patch** after that tag (e.g. latest tag `v0.1.62` → new migration `v0.1.63`). Naming a migration off the highest *existing file* under `datamigrate/` is wrong — files there are sparse (we have `v0.1.10`, `v0.1.34`, `v0.1.35`, ... not a contiguous run), so the highest file is almost always far behind the real latest release tag. Always go to the tag. Also update `internal/build/VERSION` to the same version. Never reuse upstream's label verbatim (e.g. `v1.0.0-beta6`, `v0.2.0`) — those are on upstream's version track and will either never run or run at the wrong time for our users.
- Register any new `DataMigrator` in `NewMigrator` in version order.
- If upstream's migration assumes a clean upstream schema (columns we don't have, or columns we added that upstream doesn't know about), adapt the SQL/Go to our actual schema. A migration that `ALTER TABLE`-s a non-existent column will fail at startup.
- Never delete or reorder existing registered migrations — users on older versions still need them. Only append.

**Why:** the migration runs on startup against whatever schema and system version the user's DB holds. A migration written for upstream's schema track will either crash on our column names or silently no-op past real data, leaving the DB in a half-migrated state. The semver gate means a wrongly-tagged version either runs when it shouldn't or never runs when it should. We have been bitten specifically by guessing the version from filenames instead of tags — the next version is always `latest v0.1.x tag + 1 patch`, derived from `git tag`, nothing else.

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

1. `go build ./...` and `cd llm && go build ./...` — both must compile.
2. `golangci-lint run --timeout 10m --max-same-issues 50 ./...` and `cd llm && golangci-lint run ...` — must pass.
3. `go test ./...` and `cd llm && go test ./...` — must pass.
4. `go build -o axonhub.exe ./cmd/axonhub/`, stop the running `axonhub.exe`, start the new one. Watch startup logs for migration execution and errors — the datamigrate `Migrator` runs at boot; a failing migration surfaces here.
5. Open the app in the browser, exercise the affected pages, check for errors (console + network).
6. **Upgrade-path check:** if any Hazard 2 migration was added or modified, verify against the old-version instance kept on the local machine (a real running instance of the previous release with its own DB, not a fresh DB). Point the new binary at that old instance's DB — or start the old instance, confirm it boots, then upgrade it in place to the new binary — and confirm migrations run and the app is functional afterward. This catches the case where migrations are correct on a fresh DB but break on real old data.

If any step fails, fix before committing. A merge that compiles but fails migration on a real DB is worse than no merge — it wedges users on startup.

## Committing the merge

Per [[feedback_verify_and_commit]] and AGENTS.md: run the verification commands, then commit immediately. Use the established commit message style — `merge: resolve conflicts with upstream <short description of what was merged>`. Do not commit `.exe` binaries (see [[no_exe_in_commits]]). After the merge commit lands on the working branch, merge it back to `unstable` per [[feedback_merge_to_unstable]].

## When to escalate to the user

Pause and ask before:
- Dropping one of our shipped features entirely in favor of upstream's.
- Deleting or rewriting a registered data migration that has already shipped to users (data-loss risk).
- Renaming a column or field that requires a destructive schema change.
- A conflict where the right call genuinely depends on product intent, not mechanical correctness.
