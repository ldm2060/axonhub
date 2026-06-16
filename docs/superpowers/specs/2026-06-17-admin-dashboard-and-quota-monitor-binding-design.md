# Admin Dashboard Scroll Fix and Quota Monitor Binding Design

## Summary

Fix the admin dashboard so it only scrolls to real content height, and add channel-level quota monitor binding so a channel can be temporarily excluded from routing when bound quota monitors meet user-defined exhaustion conditions.

The quota feature is edited from the channel page, summarized from the quota monitor page, and enforced through the existing `Channel.quotaBindingReady` routing guard instead of changing the channel `status`.

## Goals

- Fix `/admin` dashboard infinite/empty scrolling by making the page height follow actual content.
- Add a channel edit setting for quota monitor bindings.
- Let users configure quota exhaustion by:
  - monitor status rules, and
  - structured field conditions.
- Let each channel choose how multiple bindings aggregate:
  - `any`: disable routing when any binding matches.
  - `all`: disable routing only when all effective bindings match.
- Show read-only binding summaries on the quota monitor page.
- Preserve existing data through database migration and verify local upgrade behavior.

## Non-goals

- Do not execute arbitrary JavaScript or expression strings.
- Do not add complex nested AND/OR condition groups in the first version.
- Do not add a quota trigger history/audit page.
- Do not set the channel `status` to `disabled` for quota exhaustion. The route selector should use `quotaBindingReady=false` for temporary exclusion.
- Do not make the dashboard fixed-height or truncate dashboard cards.

## Current Context

The codebase already has several pieces that should be reused:

- `Channel.quotaBindingReady`: aggregated quota-ready state used to exclude a channel from routing.
- `Channel.quotaMultiMonitorStrategy`: current `any` / `all` aggregation strategy.
- `UsageMonitorChannel`: stores quota monitor configuration, poll result, quota status, and existing threshold-based auto-disable fields.
- `UsageMonitorService.evaluateAndUpdateChannelQuotaReady`: updates channel quota-ready state after monitor changes.
- Channel edit UI already has advanced behavior sections such as client restriction and auto-disable configuration.
- Usage monitor UI already has monitor cards and edit dialogs with parsed/display fields.

## Design

### 1. Admin Dashboard Scroll Fix

The admin dashboard route continues to render the shared `Dashboard` component. The fix should be a focused frontend layout change:

- ensure the dashboard wrapper uses natural document flow height;
- remove or adjust any parent/child `flex`, `min-h`, or `overflow` combination that creates scrollable blank space;
- keep charts and cards fully visible;
- keep personal dashboard behavior consistent.

Acceptance criteria:

- `/admin` scrolls only to the bottom of real dashboard content;
- there is no large blank area after the dashboard content;
- the page cannot continue scrolling through empty space;
- `/` personal dashboard does not regress.

### 2. Quota Binding Data Model

Use a durable backend-owned binding model rather than frontend-only state.

Preferred implementation is a dedicated binding structure if existing `UsageMonitorChannel.channel_id` cannot express the required many-to-many behavior cleanly. A suitable model is `ChannelUsageMonitorBinding` with fields equivalent to:

- `channel_id`: target channel;
- `usage_monitor_channel_id`: bound quota monitor;
- `enabled`: whether this binding participates in evaluation;
- `trigger_statuses`: list of quota statuses that mean exhausted for this binding;
- `conditions`: list of structured field conditions;
- optional `last_triggered_at` and `last_trigger_reason` if they can be added without broadening scope.

The existing `Channel.quotaMultiMonitorStrategy` remains the channel-level aggregation setting. The final routing state remains `Channel.quotaBindingReady`.

If the current `UsageMonitorChannel.channel_id` relationship is retained during implementation, it must still support the approved behavior: a channel can have multiple monitor bindings, rules can be edited from the channel page, and a quota monitor can show all affected channels. If that is awkward or lossy, use the dedicated binding table.

### 3. Condition Semantics

Each enabled binding can have two kinds of exhaustion triggers.

#### Status rules

The user can select any of:

- `available`
- `warning`
- `exhausted`
- `unknown`

A binding matches when the monitor's current `quotaStatus` is in the selected set.

#### Field conditions

Field conditions are stored as structured data, not executable expressions:

```ts
{
  field: "remaining",
  operator: "<=",
  value: "0"
}
```

Supported first-version operators:

- numeric/text equality: `=`, `!=`
- numeric comparison: `<`, `<=`, `>=`, `>`
- text containment: `contains`, `not_contains`

Field values are resolved from monitor parsed data / last poll data using stable field keys. Numeric operators require both sides to parse as numbers; if parsing fails, that condition is not matched and should produce a readable evaluation reason for diagnostics.

#### Combining conditions

Within one binding:

- status rules and field conditions use OR;
- multiple field conditions also use OR;
- an enabled binding with no status rules and no field conditions is treated as ineffective and must not disable the channel.

Across bindings on one channel:

- `any`: channel is not ready when any effective enabled binding matches;
- `all`: channel is not ready only when every effective enabled binding matches;
- no enabled/effective bindings means the channel is ready.

### 4. Backend Evaluation Flow

Re-evaluate affected channels when:

1. a quota monitor poll succeeds and updates parsed quota data;
2. a quota monitor poll fails or becomes unknown/error;
3. a channel's quota binding configuration is saved;
4. a quota monitor is enabled, paused, deleted, or otherwise made ineffective.

Evaluation result handling:

- matched exhaustion: set `quotaBindingReady=false`;
- recovered: set `quotaBindingReady=true`;
- when the channel `status` is still `enabled`, write a quota-specific `errorMessage` such as `Quota monitor triggered: Monitor A status exhausted / remaining <= 0`;
- when the channel is already disabled, do not overwrite error messages owned by manual disable, channel auto-disable, or all-API-keys-disabled logic;
- refresh the enabled-channel cache after quota-ready changes so routing responds immediately.

### 5. Frontend UI

#### Channel edit dialog

Add a quota monitor binding section near existing advanced channel behavior settings.

The section contains:

- an enable/disable control for quota binding participation;
- channel aggregation strategy selector:
  - any monitor matches;
  - all monitors match;
- a binding list where each item can:
  - select a quota monitor;
  - enable/disable that binding;
  - choose trigger statuses;
  - add/edit/remove field conditions;
  - show a human-readable condition summary.

The form must read and write all fields needed for edit echo, refresh, and list/detail display. This follows the project rule that create/edit forms must update write operations and read operations together.

#### Quota monitor page

The quota monitor page remains read-only for binding configuration. It should show a concise summary per monitor:

- affected channel names;
- each channel's aggregation strategy;
- trigger status and field condition summary;
- current matched/not-matched state if available.

Editing remains in the channel dialog to keep ownership clear.

#### I18n

Add all new UI strings to both English and Chinese locale files. Expected files are the existing `channels.json` and `usage-monitor.json` locale groups unless implementation discovers a more specific existing namespace.

### 6. Database Migration

Any database schema change must include migration support.

If a dedicated binding table is introduced, the migration must:

- create the table and indexes;
- set safe defaults for new rows/fields;
- migrate existing `UsageMonitorChannel.channel_id` relationships into binding rows;
- default migrated bindings so they do not unexpectedly disable existing channels unless the previous configuration already represented an active auto-disable rule;
- preserve `Channel.quotaBindingReady` as ready for channels with no effective conditions;
- be safe to run on databases that already contain partial upgraded state.

If fields are added instead of a new table, migration must still set defaults and preserve existing behavior.

The target release version should follow project memory: release/migration work targets `v0.1.10` labels where applicable.

### 7. Local Upgrade Verification

Implementation is not complete until local upgrade behavior is verified.

The upgrade verification should prepare or use a database representing the old state with:

- at least one existing channel;
- at least one existing usage monitor;
- at least one existing `UsageMonitorChannel.channel_id` relationship if the current schema has it;
- at least one channel with no monitor binding.

After starting the new code and running migrations, verify:

- migration completes successfully;
- old channel-monitor relationships are preserved in the new binding representation;
- channels without effective rules remain `quotaBindingReady=true`;
- no existing channel is accidentally excluded from routing;
- saving a new status rule or field condition from the channel page persists and reopens correctly;
- triggering and then clearing the condition updates `quotaBindingReady` as expected.

If the project already has migration test utilities, automate this. Otherwise, provide a repeatable local verification script or documented command sequence and include the observed result in the final report.

### 8. Tests

Backend tests should cover:

- status rule match and non-match;
- field condition numeric comparison;
- field condition text comparison;
- invalid numeric values not matching;
- empty rules not disabling;
- `any` aggregation;
- `all` aggregation;
- recovery from not-ready to ready;
- migration of existing `channel_id` relationships if a new table is added.

Frontend validation should cover or manually verify:

- channel edit dialog loads binding config;
- channel edit dialog saves binding config;
- condition summary text is correct;
- quota monitor page shows affected channel summaries;
- admin dashboard no longer has empty infinite scrolling.

### 9. Verification and Commit Requirements

Before implementation completion:

- run the applicable backend tests for quota binding and migration;
- run the project-required pre-commit verification commands when committing implementation changes;
- after backend Go changes, rebuild/restart/verify in browser according to project instructions;
- report any verification command that cannot run locally, with the concrete reason.

The implementation report must include:

- migration file(s);
- data model changes;
- tests added/updated;
- local upgrade verification result;
- browser verification result for `/admin`, channel edit binding, and quota monitor summary.
