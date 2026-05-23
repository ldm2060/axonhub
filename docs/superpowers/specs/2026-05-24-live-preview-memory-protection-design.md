# Live Preview Memory Protection Design

## Context

Live preview stores streaming response chunks in memory so request detail pages can replay already-received chunks and continue receiving new chunks over SSE. The current chunk limit is based on chunk count, but chunk sizes vary widely. Small text chunks may be cheap, while large JSON/tool-call chunks can make the same chunk count consume hundreds of MB.

This design protects the live preview buffer without changing request forwarding, provider streaming, final request status updates, or existing storage-policy behavior for request bodies, response bodies, and persisted chunks.

## Goals

- Bound live preview memory by bytes, not only by chunk count.
- Make the default global live preview budget 10% of detectable memory.
- Trigger cleanup when memory usage approaches 80%.
- Expire buffers one hour after their last appended chunk.
- Ensure memory protection never interrupts the actual request stream.
- Avoid reporting a memory-protected preview as normally completed.

## Non-goals

- Distributed live preview across multiple AxonHub instances.
- Changing provider streaming behavior.
- Changing final response persistence semantics.
- Changing `StoreRequestBody`, `StoreResponseBody`, or `StoreChunks` behavior.

## Policy semantics

The policy applies only when live preview is enabled. If live preview is disabled, buffers are not created and current behavior remains unchanged.

Each active streaming request and request execution may have an in-memory preview buffer. Every live preview buffer contributes to a per-process global live preview budget. Multi-instance deployments do not coordinate this budget; each instance manages only the buffers it owns, matching the current single-instance live preview contract.

The default global budget is 10% of detectable memory. Detection should prefer container or cgroup memory limits, then system memory. If memory cannot be detected, AxonHub should use a conservative fallback budget and log a warning. A configured explicit byte limit should override automatic detection.

Buffer expiration is based on `lastAppendedAt + 1h`. A long stream that continues producing chunks should not be deleted solely because it was created more than one hour ago.

## Configuration

Extend the existing storage policy with live preview memory controls:

- `live_preview_memory_budget_ratio`: default `0.10`
- `live_preview_memory_pressure_ratio`: default `0.80`
- `live_preview_memory_low_watermark_ratio`: default `0.70`
- `live_preview_idle_ttl`: default `1h`
- `live_preview_max_bytes`: optional explicit byte budget override

The ratio fields control automatic budget and pressure behavior. The explicit byte budget is useful when automatic memory detection is unavailable, misleading, or too permissive for a deployment.

## Byte accounting

Each `chunkbuffer.Buffer` tracks an estimated byte count. The registry tracks total live preview bytes across all request and execution buffers.

For each appended chunk, estimate:

```text
len(Data) + len(Type) + len(LastEventID) + fixed per-event overhead
```

The fixed overhead does not need to be exact; it should be conservative enough to make the budget protective. Exact Go heap size is runtime-dependent, so the budget is an operational guardrail rather than a byte-perfect profiler.

## Append behavior

Before appending a chunk:

1. Reject the append if the buffer is closed or already truncated.
2. Estimate the new chunk size.
3. If adding it would exceed the global live preview budget, run cleanup.
4. If cleanup brings usage back under budget, append the chunk.
5. If usage still exceeds budget, mark the buffer as truncated and stop retaining additional preview chunks for that stream.

Truncating a preview buffer must not stop the real stream. The provider response continues to flow through the normal pipeline, and final request status/persistence continues independently.

## Cleanup algorithm

Cleanup runs from append-time budget checks and from the existing background sweeper.

Cleanup order:

1. Remove closed buffers.
2. Remove buffers whose `lastAppendedAt + live_preview_idle_ttl` has expired.
3. Under memory pressure, remove the least recently updated buffers until usage falls below the low watermark or the preview budget is satisfied.

Memory pressure means detected process/system memory usage is at or above `live_preview_memory_pressure_ratio`. Cleanup should target the low watermark to avoid repeated cleanup on every append.

## Preview response semantics

The preview API must distinguish normal completion from memory-protection cleanup.

- Normal stream completion emits `preview.completed`.
- Memory budget truncation, memory-pressure eviction, or idle expiration emits or returns a distinct unavailable/truncated state.
- A processing request whose buffer was removed should not be reported as completed.

The exact event name can be finalized during implementation, but the contract should expose a distinct state such as `preview.truncated` or `preview.unavailable`, with a reason like `memory_budget`, `memory_pressure`, or `idle_ttl`.

## Data flow impact

Live preview cleanup affects only preview replay state. It does not affect:

- The outbound provider stream.
- The inbound client stream.
- Aggregation used by persistent stream wrappers.
- `UpdateRequestCompleted` / `UpdateRequestExecutionCompleted`.
- `StoreResponseBody` and `StoreChunks` decisions.

If the persistent stream wrappers still accumulate chunks separately, that memory remains outside the live preview budget. This design prevents live preview from adding unbounded pressure, but it does not solve all streaming accumulation memory use.

## Testing plan

- Unit test chunk byte accounting and budget rejection in `chunkbuffer`.
- Unit test that truncated buffers no longer retain additional chunks.
- Unit test registry global byte accounting across request and execution buffers.
- Unit test cleanup order: closed, expired, then least recently updated.
- Unit test pressure cleanup down to the low watermark.
- API/SSE test that evicted or truncated previews do not emit `preview.completed`.
- Regression test that real streaming and final request completion still work after preview buffer truncation.

## Implementation decisions

Use `preview.truncated` as the distinct SSE event for a live preview stream that was stopped by memory protection. Static fallback responses should use mode `preview-unavailable` and include a reason such as `memory_budget`, `memory_pressure`, or `idle_ttl`.

Keep `live_preview_max_bytes` as a backend storage-policy field in this implementation. Admin UI exposure can be added later if users need manual tuning from the settings page.
