# Merge upstream `looplj/axonhub` — 2026-09-26

**Range:** merge base `5653bf0a` → upstream tip `71783ca5` (23 upstream commits)
**Branch:** `unstable` (fork `ldm2060/axonhub`)

## Upstream features brought in

- `feat(sidebar): add customizable sidebar menu` (#2530) — nav moved to `frontend/src/config/nav-items.ts`,
  `useRawNavGroups()` + `applyHiddenNavItems()` + `sidebarPrefsStore`, customize dialog, auth redirect via
  `pickFallbackNavUrl`.
- `feat(channels): auto-detect relay endpoints and show endpoint column` (#2512) + `fix(channels): show Gemini
  endpoint protocol` (#2558) — `channel_endpoint_detect.go`, `DetectChannelEndpoints` GraphQL mutation, relay
  protocols lib, endpoint protocol column.
- `feat(quota): support multi-key Zhipu channels and GLM coding plan credit limits` (#2482) — per-account
  `QuotaLimitStatus.Account`/`AvailabilityGroup` with JSON round trip, worst-account-per-window cell,
  `renderWindowRows` in the popover.
- `feat(requests): record and show the channel API key index used per execution` (#2550) —
  `request_executions.channel_api_key_index`, requests column `密钥`/`Key`.
- `feat(requests): add upstream model consistency audit` (#2509) — `request_executions.upstream_model_id`,
  audit icon + tooltips.
- `feat(systemone): add native system one api format and typesafe jev provider support` (#2524) —
  new `typesafe` channel type, `api_format typesafe/systemone`, `llm/transformer/typesafe`, `/typesafe/v1/systemone`
  and `/v1/systemone` routes, `RequestTypeSystemOne`.
- `feat(models): persist reasoning levels and export a Pi models.json provider` (#2551) — `ModelCard.ReasoningEfforts`,
  apikeys dialog Pi tab.
- `feat(modelscope): add native async image generation and editing` (#2534) — `modelscope/image_generation`
  provider format, `image.go`.
- `feat(codex): GPT-6 Sol/Luna defaults + fast model aliases` (#2527, #2555).
- `feat(playground)`: remember last selected model (#2556), viewport-constrained layout (#2554).
- Various fixes: reasoning-only assistant content (#2552), textual Responses statuses (#2553), anthropic/gemini
  fail-closed (#2520), parallel tool use (#2523), cline quota freshness (#2495), OpenCode Go popover (#2525),
  dashboard fastestModels LEFT JOIN (#2521), cleanup-sqlite.sh robustness (#2526), model developer sync (#2531).

## Hazards & decisions

### Hazard 1 — fields we modified that upstream also changed

- Import-block conflicts (`orchestrator/outbound.go`, `orchestrator/request_execution.go`,
  `biz/channel_llm.go`, `biz/provider_quota/zhipu_checker.go`, `llm/transformer/anthropic/outbound_convert.go`):
  kept our module path `github.com/ldm2060/axonhub` and merged upstream's new imports
  (`llm/modelname`, `typesafe`, `fmt`, `llm/transformer`, `samber/lo`, `internal/log`).
- `request_execution.go`: upstream restructured the `OnOutboundRawRequest` body; kept our extra
  `reasoningEffort` capture (our fork's `CreateRequestExecution` signature) and took upstream's
  `candidate`/`entry` placement.
- **Upstream's new files all arrived with `github.com/looplj/...` imports** (24 files incl. tests, plus
  `gqlgen.yml`). Rewrote every occurrence to `ldm2060` across the tree; `gqlgen.yml` also gained upstream's
  `DetectChannelEndpoints*` model mappings on our paths.
- `internal/server/biz/request_channel_api_key_index_test.go`: upstream's test called
  `CreateRequestExecution` without our `reasoningEffort` argument — adapted the 4 call sites (pass `""`).
- `internal/ent/*` generated files were regenerated with `make generate` (ent + gqlgen) from the merged
  schemas, which added `channel.TypeTypesafe`, `request_executions.channel_api_key_index` and
  `upstream_model_id`; no gqlgen stubs left unimplemented.

### Hazard 2 — migrations

**No-op this merge.** Upstream changed nothing under `internal/ent/migrate/datamigrate/` or
`internal/build/VERSION` in this range (verified by diffing base→tip for those paths). Our files
(`v0.1.10/34/35/63/67/68`, `v0.2.8`, `predropmigrate/`) are untouched; `internal/build/VERSION` stays
`v0.2.12`. No renames, no new registrations.

Standing divergence (unchanged, pre-existing): upstream's `v0.3.0`/`v0.4.0` datamigrations are absent in our
tree (dropped in an older merge; upstream's onboarding flow differs from our first-run initialization). Worth a
conscious review if upstream ever makes those load-bearing for existing installs.

### Hazard 3 — overlapping features

- **channels columns**: our fork extracts cells into `channels-column-cells.tsx`; upstream kept them inline and
  added the endpoint protocol column and quota collapse helpers. Kept our structure and ported upstream's
  behavior (`EndpointProtocolsCell`, `collapseQuotaLimitsByWindow`, `quotaWindowLabel`, account labels).
- **sidebar/nav**: adopted upstream's `nav-items.ts` + prefs structure with **our** fork routes and items
  (`/admin/*`, `/my-channels`, `/my-models`, usage monitor, publish requests, prompt protection, system).
  `playground` and `usage-stats` entries stay hidden (deliberate fork commits `1df29c3a`, `83f54e0b`).
- **auth.ts**: kept our Turnstile/sign-up additions; adopted upstream's hidden-nav-aware redirect with our
  non-owner landing (`/project/requests`).
- **quota-badges**: adopted upstream's `renderWindowRows` multi-account structure, kept our fork tweaks.
- **providers catalogs** (frontend `models/data/providers.json` and backend `catalogdata/providers.json`):
  upstream's synced data is a strict superset — took upstream's content verbatim.
- **channels-action-dialog**: format selector hidden for `jina|codex|claudecode|zcode|typesafe`.
- **provider_quota_cache_test.go**: stays deleted (fork refactor `26cde777` removed the old cache tests);
  upstream's new regression test was ported into `provider_quota_account_limits_test.go`.

## Verification (all green)

- Go: `go build ./...` and `cd llm && go build ./...`; `golangci-lint` 0 issues both modules (cache cleared);
  `go test ./...` both modules.
- Frontend: `pnpm lint`, `pnpm format:check`, `pnpm typecheck`, `pnpm test:unit` (205 pass), `pnpm build`,
  `pnpm bundle:check`. Prettier applied to 6 merged files that failed `format:check`.
- **Upgrade path on the real old DB** (`profiling_web_axonhub.db`, `system_version=v0.2.12`): merged binary
  booted, pre-drop migration v0.1.60 ran, all datamigrations correctly skipped by the semver gate, Ent added
  `channel_api_key_index` + `upstream_model_id` in place; row counts unchanged (13 channels / 52 executions /
  46 requests).
- Browser (vite 5173 → 8090, real data): dashboard, channels (endpoint protocol icons active/dimmed), requests
  (`密钥` column, model audit tooltips), request detail (upstream-reported model, key suffix, response headers),
  sidebar customize dialog (19 items), playground (selection persisted in localStorage), models, API key dialog
  (6 tabs incl. Pi models.json). No console errors or error boundaries anywhere.
