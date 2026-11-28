# Fork-only feature inventory — merge of `remote/unstable` @ `043fa620b` (37 upstream commits, 2026-11-28)

Generated per merge-upstream skill **Hazard 0**. Every anchor below must still resolve in the
merged tree after conflict resolution. Verify with `git grep -l <anchor>` — an anchor with zero
hits is a silently dropped fork feature.

## This merge (2026-11-28): upstream highlights

Plugin system: community plugin hosting (#3088) + registry read from axonhub-plugins, managed
tables + page access (#3086), scheduled tasks + dynamic SQL (#3080), task GraphQL + nav badges
(#3095), gateway server middleware (#3069), read-only model catalog checks (#3110), jev-guard
maxTurns (#3103) and fail-blocked default (#3104). Realtime: OpenAI + Bailian/Qwen WebSocket
gateway (#3051) with coder/websocket transport (#3091), WebSocket lanes (#3094/#3096/#3098),
prefer configured Responses WebSocket (#3079), structural migration `biz.MigrateChannelRealtime`.
Channels: 统一渠道限流状态 (#3101), Vertex mixed API key + service account credentials (#2855),
Claude Code OAuth model fetch (#3061), default exchange rate for model prices (#3060), bulk
import global models + reference price sync (#3067). Also: Codex compatibility config blocks
split (#3081), watermark prompt_extend (#3082), moonshot K2.6/K2.7 Code/K3 sampling omission
(#3097), ollama seed/penalties (#3083) + tool_calls finish_reason (#3064), Gemini final
thought-signature chunk (#2721), Codex Guardian review linking (#3078), Go 1.27.4 bumps.

## This merge (2026-11-28): decisions log

- **Module path**: 91 upstream files rewritten `looplj`→`ldm2060`; `go.mod`/`llm/go.mod` taken from upstream with the fork's `afero-s3`/`afero-webdav` replaces restored.
- **Hazard 2**: upstream added NO version-gated DataMigrator. `datamigrate/migrator.go` gained an unconditional structural call `biz.MigrateChannelRealtime` (idempotent, runs even when the stored version is current) + new `plugin_schedules`/`plugin_schedule_runs` tables via ent schema. No version rename needed (upstream VERSION still `v1.0.0-beta9`; ours stays `v0.2.43` until the release bump).
- **System entity GraphQL exposure (Hazard 1)**: upstream's `entgql.Skip(SkipAll)` on `internal/ent/schema/system.go` was reverted to the fork's `QueryField+RelayConnection+Mutations` — ours' generated gql trio depends on the `systems()` query, and restoring it keeps ent graph consistent. Nothing in the fork UI uses `systems()` today, but regenerating without it invalidated ours' generated.go.
- **APIKey.key where-input (Hazard 1)**: upstream added `SkipWhereInput` to the API key field; the fork's API-keys page searches by key (`keyContainsFold` OR-search) so the skip was dropped to keep the fork feature.
- **`conf/conf.go`**: kept the fork's 120s `llm_stream_idle_timeout` default (upstream's 600s), took upstream's new `server.realtime.*` defaults.
- **Responses WebSocket (Hazard 3 hybrid)**: kept the fork's admission skeleton in `processMessage` (copy-commit `nextSession` + `AcquireWebSocketConcurrency` gate + reservation undo) and adopted upstream's budget accounting (transient `session.mergedBytes`, refund-at-exit defer, `WithResponsesWarmupAccepted`, conditional lease retirement) and coder/websocket `CloseNow` lifecycle. Fork's `startKeepaliveProbe` kept, translated gorilla→coder (`conn.Ping`); fork's gorilla `dial` dropped (upstream's 3-return dial lives in the new `websocket_dial.go`). `pooledWebSocketConn` carries BOTH upstream's `receiver/activityConn` and the fork's `done` channel (probe stop) — the close path calls `receiver.close()` then `close(done)`.
- **llm tests**: gorilla-based fork tests (`websocket_executor_test.go`, lane test) translated to coder/websocket API; fork's `Upgrader`/`WriteJSON`/`WriteControl` gone.
- **gqlgen bootstrap (two passes)**: the gql package cannot load until generated files match BOTH sides' resolvers. Pass 1: moved `plugin.resolvers.go plugin_view.go plugin_ui.go axonhub.resolvers.go model.resolvers.go ent.resolvers.go` out, ran gqlgen (regenerates `generated.go`/`models_gen.go` from the merged `.graphql` superset), restored the merged resolver files, ran pass 2 (stubs missing methods like `Systems`). Added `configurePluginGraphQL`/`graphqlTransactions` helpers to `graphql.go` (upstream's refactor, file is handwritten).
- **models-table grouping (React Compiler trap)**: upstream's `groupedRows`/`filteredSelectedRows` `useMemo([table])` froze on the initial empty data (tanstack v8 table instance is stable; the fork disables React Compiler) — reverted to the fork's recompute-every-render IIFE. This was caught only by browser verification, not by any gate.
- **models-catalog-prices.tsx (react-refresh)**: non-component exports (`selectStyle`, `offerKey`, `useReferencePriceFormat`, `defaultSelection` + `findOffer/needsRebind/defaultRequest` helpers) moved to the new `models-catalog-price-format.ts` (fork's established split pattern); `highlightMaskedCode` added to `code-highlight.ts` (delegating to the fork's `highlightCode`) because the fork's masked-code-block lacks upstream's export.
- **channels-columns**: restored the fork's extracted-cell structure (import from `channels-column-cells.tsx`); ported upstream's realtime provider badge + `getChannelRelayProtocols(..., settings?.realtime?.enabled)` into the fork's cells; the fork's StatusSwitchCell/QuotaCell already carried upstream's status dialog and quota-collapse features.
- **channels/data/channels.ts**: every query keeps BOTH the fork's `pooling` selection and upstream's `realtime` selection.
- **locales**: JSON-level merge, fork keys win, upstream additions kept (`models.import.*`, `channels.endpoints.qwenRealtime.*`, …).
- **users**: upstream's derived `canEdit` replaced the fork's `canEdit/setCanEdit` state; `isAdmin` checkbox gated by `isOwner` (mirrors backend `CanGrantAdmin`); `RotateAPIKey` is upstream's structure with the fork's broader personal-key rule (creator, owner, or system `write_api_keys` holder); `model_fetcher.FetchModels` keeps fork's KimiCode/ZCode branches over upstream's `previewModels` default.
- **models-provider rename**: fork's `models-provider.tsx` + `models-context.ts` layout kept; upstream's new `catalogImport` dialog type added.
- **endpoints dialog**: taken from upstream wholesale (it is a superset — includes the fork's Codex WebSocket logic); fork's direct-path `@lobehub/icons/es/Google/components/Mono.js` import restored (the barrel import breaks the build — lobehub-ui 5.39 dropped `Divider`, v0.2.27-class incident).
- **plugin parity**: `forkUpstreamParityVersion` bumped `1.0.0-beta9`→`1.0.0-beta10` (this merge tracks upstream's unreleased task/table/middleware surface).
- **Bundle budgets**: `modelsDialogs` 100→110 KiB raw / 25→27 KiB gzip with a dated comment.
- **Tests adapted to merged APIs**: upstream's new `realtime_routing_test.go`/`realtime_reconnect_test.go` pass `nil` PoolingGate; fork's api ws tests translated gorilla→coder; `CreateRequestExecution` call sites pass `""` reasoningEffort; `skipGraphqlTransaction` tests updated to the `*ast.OperationDefinition` signature; `channel-realtime.test.mjs`/`protocol-options.test.mjs` regexes adapted to prettier-formatted sources and the fork's extracted-cells file.
- **Known pre-existing (not from this merge)**: Dockerfile ldflags still carry `github.com/looplj/axonhub/...` version stamp (no-op on the fork; goreleaser sets the real values); `biz/email.go` IPv6 dial vet note; `stream_lifecycle_test.go` lostcancel note. Left untouched.

## Post-resolution anchor verification (merge of 2026-11-28, `043fa620b`)

All anchors re-checked in the merged tree, all present: `OwnerRule` 26, `CanRouteThroughChannel` 10,
`RouteGuard` 34, `enableSameChannelRetry` 7, `shared_with` 30, `myUsageStatsByUser` 4,
`manage_own_channels` 10, `usePresenceExitGuard` 3, `IsAdmin` 21, `CanGrantAdmin` 2,
`auth.signIn.links.signUp` in both base.json locales, fork ent entities (channelusagemonitorbinding,
apikeyprofiletemplate, channeloverridetemplate, publishrequest, datastorage, userusagestats) all present,
fork-only frontend files (access/pooling/zcode/share/route-guard/apikeys-context) all present,
`usage_monitor_visibility.go` + `channel_pooling.go` present in `internal/server/biz/`.

Browser verification (local profiling deploy, port 8090): dashboard, channels table, models grouped
table (3 developers · 6 models with fork labels), users table, plugins page, api-keys masked cells,
usage-monitor panels all render with no console errors after the models-table grouping fix.

## Prior merge (2026-11-21, `c2826aa42`): decisions log

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
| Admin identity decoupled from scopes (579199e05) | `IsAdmin`, `CanGrantAdmin` |
| Channel pooling member share by API-key ownership (2b386e950) | `channel_pooling.go` |
| Battery panel monitor visibility (ed64579f5) | `usage_monitor_visibility.go` |
| Usage monitor estimate windows (2109c8710 et al.) | `estimateWindows`, `estimate` |
| Kimi-code reverse proxy (0a63f3a5d) | `kimicode.go` |
| Radix exit-animation guard (60f388e8f) | `usePresenceExitGuard` |
| OIDC http redirect URIs (5113f3cbf) | `allowHttpRedirect` |
| manage_own_channels edit-own (21439cc28) | `manage_own_channels` |
| Desktop updater key gating (47628fa78) | `desktop-release.yml` updater signing |
| Public mail token privacy bypass (94a8fdd53) | `WithSystemBypass` |
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
