# Quota-Channel Binding Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec**: `docs/superpowers/specs/2026-06-15-quota-channel-binding-design.md`

**Goal:** Enable automatic channel availability management based on quota monitoring.

**Architecture:** UsageMonitor polling evaluates auto-disable → updates quota_ready → aggregates to Channel.quota_binding_ready → orchestrator filters.

**Tech Stack:** Go 1.26+, Ent ORM, React 19, TypeScript, GraphQL

---

## Phase 1: Foundation (Backend Schema & Config)

### Task 1: UsageMonitorChannel Schema
- [ ] Add `auto_disable_enabled`, `auto_disable_threshold`, `auto_enable_threshold` fields to `internal/ent/schema/usage_monitor_channel.go`
- [ ] Run `go generate ./internal/ent`
- [ ] Verify: `go build ./internal/ent`
- [ ] Commit

### Task 2: Channel Schema  
- [ ] Add `quota_binding_ready`, `quota_multi_monitor_strategy` fields to `internal/ent/schema/channel.go`
- [ ] Run `go generate ./internal/ent`
- [ ] Verify: `go build ./internal/ent`
- [ ] Commit

### Task 3: Configuration
- [ ] Add `QuotaChannelBindingConfig` struct to `conf/conf.go` (defaults: 1.0, 0.95, "any")
- [ ] Add `QuotaChannelBinding` field to `Conf` struct
- [ ] Add default initialization logic
- [ ] Verify: `go build ./conf`
- [ ] Commit

### Task 4: Data Migration
- [ ] Create `internal/ent/migrate/datamigrate/v0.1.10.go` with ALTER TABLE statements
- [ ] Create `internal/ent/migrate/datamigrate/v0.1.10_test.go` with migration tests
- [ ] Run migration tests
- [ ] Commit

---

## Phase 2: Core Logic (Backend Business Logic)

### Task 5: Helper Functions
- [ ] Create `internal/server/biz/usage_monitor_quota_binding.go`
- [ ] Write tests first: `TestCalculateMaxUsageRatio`, `TestEvaluateQuotaReady`, `TestGetThresholds`
- [ ] Implement: `calculateMaxUsageRatio()`, `evaluateQuotaReady()`, `getDisableThreshold()`, `getEnableThreshold()`
- [ ] Run tests, verify pass
- [ ] Commit

### Task 6: Monitor Polling Extension
- [ ] Modify `internal/server/biz/usage_monitor.go::pollUsageMonitorChannel()`
- [ ] Add auto-disable evaluation after deriving quota status
- [ ] Update monitor.quota_ready when threshold crossed
- [ ] Log state changes
- [ ] Call `evaluateAndUpdateChannelQuotaReady()` if builtin source
- [ ] Write test: `TestPollWithAutoDisable`
- [ ] Run test, verify pass
- [ ] Commit

### Task 7: Channel Aggregation
- [ ] Modify `internal/server/biz/usage_monitor_internal.go`
- [ ] Implement `evaluateAndUpdateChannelQuotaReady(ctx, channelID)`
- [ ] Query active monitors with auto-disable enabled
- [ ] Get channel's strategy (fallback to global default)
- [ ] Aggregate based on "any"/"all" strategy
- [ ] Implement `updateChannelQuotaBindingReady()` and `buildErrorMessage()`
- [ ] Write tests: `TestEvaluateChannelQuotaReady_Any`, `TestEvaluateChannelQuotaReady_All`
- [ ] Run tests, verify pass
- [ ] Commit

---

## Phase 3: Integration (Orchestrator)

### Task 8: Orchestrator Filtering
- [ ] Modify `internal/server/orchestrator/candidates_quota.go::ProviderQuotaSelector.Select()`
- [ ] Add filter: exclude channels with `quota_binding_ready=false`
- [ ] Write test: `TestProviderQuotaSelector_WithQuotaBinding`
- [ ] Run test, verify pass
- [ ] Verify integration: `go test ./internal/server/orchestrator/...`
- [ ] Commit

---

## Phase 4: Frontend (GraphQL & UI)

### Task 9: GraphQL Schema
- [ ] Add fields to `internal/server/gql/schema/usage_monitor_channel.graphql`
- [ ] Add fields to `internal/server/gql/schema/channel.graphql`
- [ ] Run `go generate ./internal/server/gql`
- [ ] Verify: `go build ./internal/server/gql`
- [ ] Commit

### Task 10: UsageMonitor Form
- [ ] Modify `frontend/src/features/usage-monitor/components/UsageMonitorForm.tsx`
- [ ] Add "Auto-Disable" section with toggle, threshold inputs
- [ ] Add form validation (disable >= enable)
- [ ] Test in browser
- [ ] Commit

### Task 11: Channel List Badge
- [ ] Modify `frontend/src/features/channels/components/ChannelList.tsx`
- [ ] Add "Quota Disabled" badge when `quotaBindingReady=false`
- [ ] Test in browser
- [ ] Commit

### Task 12: Channel Form
- [ ] Modify `frontend/src/features/channels/components/ChannelForm.tsx`
- [ ] Add "Quota Binding Settings" section with strategy selector
- [ ] Test in browser
- [ ] Commit

### Task 13: Channel Detail Page
- [ ] Modify `frontend/src/features/channels/components/ChannelDetail.tsx`
- [ ] Add "Bound Usage Monitors" card with table
- [ ] Add alert for quota-disabled state
- [ ] Test in browser
- [ ] Commit

---

## Phase 5: Comprehensive Testing

### Task 14: Unit Tests
- [ ] Create `internal/server/biz/usage_monitor_quota_binding_test.go`
- [ ] Write all 7 test cases from spec (threshold exceeded, usage drops, strategies, disabled, global default, status change)
- [ ] Run: `go test ./internal/server/biz/...`
- [ ] Verify all pass
- [ ] Commit

### Task 15: Integration Tests
- [ ] Add end-to-end test in `internal/server/biz/usage_monitor_integration_test.go`
- [ ] Test: create monitor → high usage → verify filtered → low usage → verify included
- [ ] Run test
- [ ] Commit

### Task 16: Build & Verification
- [ ] Run full backend build: `go build ./...` and `cd llm && go build ./...`
- [ ] Run all tests: `go test ./...` and `cd llm && go test ./...`
- [ ] Run lint: `golangci-lint run --timeout 10m ./...` and `cd llm && golangci-lint run --timeout 10m ./...`
- [ ] Build frontend: `cd frontend && npm run build`
- [ ] Start dev server, test manually in browser
- [ ] Final commit with summary

---

## Success Criteria

✅ All schema changes applied and generated  
✅ Configuration loaded with proper defaults  
✅ Auto-disable logic correctly updates quota_ready  
✅ Channel aggregation works for "any" and "all" strategies  
✅ Orchestrator filters by quota_binding_ready  
✅ Frontend displays quota-disabled state  
✅ All tests pass (unit + integration)  
✅ Build and lint clean

---

## Notes for Implementer

- **Refer to spec**: Detailed pseudocode and logic is in `docs/superpowers/specs/2026-06-15-quota-channel-binding-design.md`
- **TDD**: Write tests first for each business logic function
- **Commit frequently**: After each task completion
- **Test incrementally**: Don't wait until the end to test
- **Edge cases**: Pay attention to nil values, empty lists, and concurrent updates
