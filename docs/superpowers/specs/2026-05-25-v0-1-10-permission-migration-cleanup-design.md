# v0.1.10 permission and migration cleanup design

## Goal

Fix role creation and personal/project page permission failures by making permission defaults consistent for both new and existing data. The next data migration version is `v0.1.10`; higher-version data migration files such as `v0.3.0`, `v0.4.0`, and `v0.5.0` should be removed from this branch's migration line.

## Scope

- Replace the current higher-version data migrations with a single `v0.1.10` migration.
- Backfill existing users and project memberships so current default permissions actually apply to existing data.
- Fix new project/private-project owner scope assignment so the bug does not recur.
- Fix frontend permission keys and missing scope translations.
- Keep authorization based on explicit scopes rather than introducing frontend-only owner bypasses.

## Backend data migration

Create `internal/ent/migrate/datamigrate/v0.1.10.go` and matching tests. Register only `NewV0_1_10()` from the data migrator.

The migration should be idempotent:

1. Add any missing `biz.DefaultUserScopes` values to existing normal users that were created before the current defaults included role and personal-management scopes.
2. Add the full private-project owner project scope set to existing private project owner memberships.
3. Add the same owner project scope set to existing project owner memberships whose direct `user_project.scopes` are empty or incomplete.
4. Preserve any existing custom scopes and never remove scopes from users or memberships.

Delete old higher-version data migration files and tests from `internal/ent/migrate/datamigrate/`, then update migrator tests that assert version behavior so they use the current `v0.1.10` line.

## Backend runtime behavior

Unify owner membership creation between private projects and normal projects. New project owners should receive the same explicit project scopes used by the backfill, including API key, request, prompt, and role scopes.

No new implicit owner shortcut should be added to backend or frontend authorization. Owners should work because their memberships carry the scopes required by existing guards and privacy rules.

## Frontend fixes

Replace stale `read_system` permission checks with the existing settings scope, `read_settings`, in route guards and permission-gated UI.

Add missing i18n labels in both English and Chinese locale files for active scopes that currently render as raw keys, including:

- `manage_own_channels`
- `manage_own_models`
- `review_publish_requests`
- `read_data_storages`
- `write_data_storages`

Enable permission filtering for project role scope selection, matching the system role dialog behavior, so users do not select scopes the backend will reject.

## Testing

- Add migration tests that cover existing users missing default scopes, private project owner memberships with empty scopes, regular project owner memberships with empty/incomplete scopes, and idempotent reruns.
- Update migration registration/version tests for `v0.1.10`.
- Add or update backend service tests for project owner scope assignment if existing coverage does not catch the issue.
- Run frontend type/lint checks if available for the permission key and i18n changes.
- Run the repository verification required by project instructions before committing implementation changes.

## Non-goals

- Do not change the permission model to make ownership an implicit wildcard permission.
- Do not remove active scopes just because their translations were missing.
- Do not keep old higher-version data migration files on this branch.
