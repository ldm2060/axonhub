# Fork-only feature inventory — merge of `remote/unstable` @ `c2826aa42` (36 upstream commits)

Generated per merge-upstream skill **Hazard 0**. Every anchor below must still resolve in the
merged tree after conflict resolution. Verify with `git grep -l <anchor>` — an anchor with zero
hits is a silently dropped fork feature.

## This merge (2026-11-21): decisions log

Upstream highlights: URL-prefix serving (#3058), scheduled disable/enable by cron (#3050),
channel model-price sync from endpoints (#3038), configurable upstream stream idle timeout
(#2905), forceArrayStop transform option (#3017), Codex catalog version in model discovery
(#3045), Copilot reasoning signatures (#3035), plus dependency bumps (TS 6, eslint 10,
i18next 27, Go module updates).

- **Streaming timeouts (Hazard 3 hybrid):** upstream's `WithLLMTimeout`/`SelectLLMTimeout`
  middleware (#2905) subsumes the fork's decode-then-select design; took upstream's wiring in
  `routes.go`/`codex_routes.go`/`api/*` and layered the fork's handler-lifetime
  `context.WithCancel` back on top in `chat.go`/`playground.go`. Kept the fork's conservative
  idle default (120s, from the fork's original feature commit) and added upstream's
  `llm_stream_request_timeout` (1800s). Removed the fork's `newHTTPServer` (WriteTimeout: 0)
  in favor of upstream's `writeTimeout()` + per-route clearing; `server_test.go` now tests
  `writeTimeout()` semantics.
- **Upstream new files** arrived with `looplj` paths; all rewritten to `ldm2060` (30 files).
  Root/llm `go.mod` taken from upstream, module path rewritten, fork's `afero-s3`/`afero-webdav`
  replaces restored (they mirror upstream's real repos), `oliveagle/jsonpath` re-added by tidy.
- **channels queries:** `availability` (fork) + `scheduledDisable` (upstream) both in every
  Channel selection; upstream's `CHANNEL_QUERY_ROUTING_STATUS_SELECTION` included in the
  composed list node.
- **sign-up page:** kept the fork's design (`auth.signUp.*` keys); upstream's version links to
  `/terms` + `/privacy` routes that don't exist in the fork.
- **invite link:** kept the fork's `/invite?token=` route (upstream moved to `/sign-up?invite=`,
  which our sign-up route doesn't accept).
- **traces:** took upstream's `buildGUID` + `appPath` URLs (base-path correct; the gqlgen GUID
  int coercion accepts both forms).
- **apikeys-view-dialog:** deps array switched to upstream's `serverURL` (base-path aware).
- **providers.json / pnpm-lock.yaml:** taken from upstream (generated data; same provider sets).

## Post-merge fixes made while verifying (2026-11-21)

- `conf_test.go`: upstream's `TestLLMTimeoutConfig` table (came with #2905) expected the 600s
  idle default; updated the three default rows to the fork's 120s.
- `request_storage_boundary_test.go`: `CreateRequestExecution` grew a `reasoningEffort` arg
  (upstream #3052); test call sites pass `""`.
- `dashboard_usage_aggregates_test.go`: `UsageStatsByUser` calls now pass the fork's
  `projectID *objects.GUID` arg as `nil`.
- `channel_scheduled_disable_test.go`: the two stale-CAS tests are nondeterministic on Windows
  (coarse ~5ms clock makes the admin edit's `updated_at` equal the snapshot's, so the
  compare-and-set wrongly fires); the admin edit now bumps `updated_at` explicitly by 1ms.
- `middleware/llm_response_writer_test.go`: `TestUnlimitedLLMBlockedDownstreamWrites` skipped
  on Windows (loopback accepts the whole 16 MiB payload without blocking).
- Frontend TS6/eslint-10 fallout (upstream's CI skips the frontend build, so these surfaced
  only locally): dropped deprecated `baseUrl` from both tsconfigs (paths-only), added
  `"types": ["node"]` for the node:test file, widened two narrow `t:` prop types to
  `TFunction`, fixed 5 `no-useless-assignment` sites, `preserve-caught-error` in
  `audio-utils.ts`, moved `CRON_PRESETS`/`CRON_PRESET_LABEL_KEYS`/`TIMEZONE_OPTIONS`/
  `zoneOffsetLabel` out of the rules editor into `channel-auto-disable-shared.ts`
  (react-refresh), updated the cron-picker guard test to the new file, removed duplicate
  framer-motion imports in the two trace views, prettier on the touched files, and raised the
  `adminModels`/`modelsProvider` bundle budgets for this merge's data growth.
- llm module lint (fork `default: all` config, stricter than upstream's): blank line between
  file header and `package` in 5 upstream-new files (godoclint), blank line after embedded
  struct fields in 5 files (embeddedstructfieldcheck).

## Fork-only source files (459 files exist in `origin/unstable` but not `remote/unstable`)

Do not let the merge delete them. Known-legitimate upstream deletions are excluded from the
"must survive" list below (they are renames or upstream removals):
`.agent/rules/*` → `.agents/rules/*` (rename, fork rule docs must survive at the new path),
`plugin/**` (removed upstream), `.trae/rules/all.md`, `.windsurf/rules/all.md`.

Fork-only Go/TS files that must survive (spot list, full list via
`git diff --name-only remote/unstable origin/unstable --diff-filter=A`):

- `frontend/src/features/channels/data/access.ts`, `shared.ts`, `pooling.ts`, `zcode.ts`, `kimicode.ts`
- `frontend/src/features/channels/components/channel-client-restriction.tsx`, `channel-pooling-settings.tsx`,
  `channel-quota-monitor-binding.tsx`, `channels-share-dialog.tsx`, `channels-personal-buttons.tsx`
- `frontend/src/features/channels/personal.tsx`, `personal-channel-source.ts`
- `frontend/src/features/apikeys/context/apikeys-context.ts`, `components/apikeys-active-profile-cell.tsx`
- `frontend/src/features/dashboard/components/user-usage-bar-chart.tsx`, `weekly-usage-line-chart.tsx`, `review-queue.tsx`
- `frontend/src/features/models/data/model-icon-*.generated.ts`, `hooks/use-developer-label.ts`
- `frontend/src/features/oauth-applications/**`
- `frontend/src/features/auth/invite/**`, `reset-password/**`, `verify-email/**`, `components/turnstile-widget.tsx`

## Anchor table (grep-able)

| Fork feature | Anchor(s) |
|---|---|
| Owner-first Ent privacy policies | `OwnerRule` |
| Channel access / routing gate | `CanRouteThroughChannel` |
| Same-channel retry setting | `enableSameChannelRetry` |
| Channel sharing | `shared_with` |
| Per-monitor / per-channel provider quota | `channel_provider_quota_hook.go`, `provider_quota/`, `zhipu_checker.go` |
| Usage monitors + header quota popover | `quota-window-display.tsx`, `usage-monitor` |
| Custom API-key profiles | `apikeys-active-profile-cell`, `apikeys-context` |
| Personal dashboard scoping | `myUsageStatsByUser`, `myDailyUsageStatsByUser` |
| Sign-up link i18n | `signIn.links.signUp` |
| Client restriction / concurrency | `clientRestriction`, `concurrencyLimit` |
| Project/module nav gating | `use-ensure-selected-project` |
| ZCode coding-plan signing | `zcode`, `coding-plan` |
| Module path | `github.com/ldm2060/axonhub` (and absence of `github.com/looplj/axonhub`) |

## Post-resolution anchor verification (merge of 2026-11-21, `c2826aa42`)

All anchors re-checked in the merged working tree, all present:

| Anchor | count (files) |
|---|---|
| `OwnerRule` | 32 |
| `CanRouteThroughChannel` | 13 |
| `RouteGuard` | 37 |
| `enableSameChannelRetry` | 11 |
| `shared_with` | 39 |
| `myUsageStatsByUser` | 7 |
| `manage_own_channels` | 14 |
| `usePresenceExitGuard` | 3 |
| `auth.signIn.links.signUp` (2 locale files) | 2 |
| Fork ent entities (`channelusagemonitorbinding`, `apikeyprofiletemplate`, `channeloverridetemplate`, `publishrequest`, `datastorage`, `userusagestats`, `usagemonitorchannel`) | all present |
| Fork-only frontend files (access/pooling/zcode/share/invite/route-guard/...) | all present |

Hazard 2: upstream added **no** migrations in this merge (`git diff HEAD...MERGE_HEAD --
internal/ent/migrate/` empty); `internal/build/VERSION` stays `v0.2.40` (upstream's
`v1.0.0-beta9` not taken).

## Fork adaptations confirmed after the merge

- Module path: `grep -rl 'github.com/looplj/axonhub' --include=*.go --include=*.mod --include=*.yml` → empty
  (147 files rewritten; `plugins/registry.json` external plugin URLs and `replace ... => github.com/looplj/*`
  mirrors deliberately left alone).
- Dockerfile frontend-builder keeps `RUN pnpm exec vite build --emptyOutDir --outDir /build/dist`.
- `cmd/axonhub-desktop/go.mod` (new upstream module) needed the fork's five `replace` directives copied in —
  a nested module does not inherit them.
- `cmd/axonhub-desktop/mygo.config.ts` update channel repointed to `ldm2060/axonhub`.
- Ent policies still begin with `scopes.OwnerRule()`; `internal/ent/schema/api_key.go` identical to ours.
- Dashboard helper implementations still live in `dashboard_cache.go` / `dashboard_channel_stats.go` /
  `dashboard_usage_aggregates.go`, not in a `*.resolvers.go`.

## Deliberate upstream-wins decisions (fork had no competing implementation)

- `react-day-picker` 9.7.0 → 10.0.2: migrated the four v9 call sites (`day_selected` → `selected`,
  `initialFocus` → `autoFocus`).
- `eslint-plugin-react-hooks` 5.2.0 → 7.1.1: the 14 new React-Compiler rules are switched off in
  `frontend/eslint.config.js` with a comment; classic `rules-of-hooks` + `exhaustive-deps` stay on.
- Anthropic unknown `finish_reason` now maps to `end_turn` (upstream #3007/#3008); the fork's
  passthrough assertions were updated — Anthropic's `stop_reason` is a closed set.
- Upstream's terminal-update response-header persistence (#2993) replaced the fork's
  `middleware.WithResponseHeaders`; the middleware and its test are deleted, and the fork's
  `stream_delivery_headers_test.go` keeps its coverage with a local writer wrapper.
- `plugin/**` (examples, dev docs, the old `plugin/scripts/pack-plugin`) deleted by upstream; the
  runtime plugin system lives in `internal/server/plugin/` and is intact.
- Pprof: upstream's `internal/server/pprof.go` (loopback-validated `addr`) replaced the fork's inline
  `Host`/`Port` server in `server.go`; `conf/conf.go` keeps only upstream's `server.pprof.addr` default.

## Fork fixes made while resolving

- `biz.ClientRestrictionChecker.CheckClientRestriction`: an unset level now means "off" instead of
  denying every channel. `normalizeRetryPolicy` only runs on the write path, so a policy read straight
  from settings could carry `""` and take routing down entirely.
- `requestResolver.RequestBody` / `requestExecutionResolver.RequestBody`: re-query the hidden storage
  metadata (`Select(FieldProjectID, FieldDataStorageID, FieldRequestBodyChunked, FieldRequestBody)`)
  as upstream's `7af565b5d` does — entgql's `collectField` only selects GraphQL-visible columns, so
  `RequestBodyChunked` was always false in resolvers and `LoadRequestBody` returned the raw manifest.
- `inbound_stream.go`: kept the fork's deferred tool-call start and layered upstream's
  `anthropicToolBlockID` normalization on top.
