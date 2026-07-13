# Requests Page Memory Leak Repair Design

Date: 2026-07-13

## Problem

The Requests UI has three memory-growth paths:

1. `useAnimatedList` appends refreshed records to an unbounded animation queue. If requests arrive faster than the animation interval drains them, the queue grows for as long as auto-refresh remains enabled. This hook is also used by Traces and Threads.
2. `RequestBodyDrawer` accumulates every adjacent page in `allRequests`. Closing the sheet leaves the drawer component mounted, so the accumulated navigation records and the last detailed request remain reachable.
3. The live request preview copies the complete `responseChunks` array for every SSE chunk. Long streams retain all chunks and cause quadratic transient allocation from repeated array copies.

The Requests toolbar also loads the complete channel summary list. This can create a large but bounded initial footprint and is required for complete filtering, so it is outside the leak fix.

## Goals

- Bound memory while auto-refresh runs under sustained request traffic.
- Bound memory while navigating through many requests in the quick-view drawer.
- Release quick-view request and response bodies after the drawer closes.
- Preserve complete live-preview output without copying the full chunk history for every event.
- Preserve current visible behavior: row animations, cross-page drawer navigation, request details, Curl preview, and live response preview.
- Apply the shared animation-queue fix safely to Requests, Traces, and Threads.

## Non-goals

- Redesigning the Requests page or filter UI.
- Limiting the channel filter to an incomplete subset.
- Globally changing React Query cache behavior.
- Truncating completed request or response content.
- Refactoring unrelated request-detail code.

## Design

### 1. Bounded auto-refresh animation state

Extract the queue reconciliation into a small pure helper so its behavior can be unit tested without a React rendering dependency.

On every server refresh:

- Remove queued records whose IDs no longer appear in the latest server page.
- Add only records that are newer than the first displayed record and are not already displayed or queued.
- Preserve chronological animation order.
- Cap the queue to `pageSize`, retaining the newest relevant records when saturation occurs.
- Continue capping displayed rows to `pageSize`.
- Clear the queue whenever auto-refresh is disabled or the server page requires a hard reset.

This bounds the hook to approximately two pages of records: one displayed page and one pending page. Under overload, obsolete intermediate animations may be skipped, but the UI converges to the latest server page rather than retaining stale records indefinitely.

### 2. Bounded drawer navigation window

Replace the indefinitely growing flat request list with a deque of at most three fetched pages. Each page keeps its own records and `pageInfo`; the drawer flattens the deque only for display. Adjacent pages are deduplicated by request ID.

When a fourth page is fetched:

- Fetching older records evicts the newest page farthest from the new current item.
- Fetching newer records evicts the oldest page farthest from the new current item.
- The current page and selected request are always retained.
- The first and last retained pages keep their original boundary cursors. Their `hasPreviousPage` and `hasNextPage` values identify evicted ranges, so reaching either end refetches that range from the server.

The page-deque logic will be extracted into a pure helper and covered by tests for deduplication, current-index adjustment, eviction, cursor preservation, and both navigation directions.

### 3. Drawer close cleanup and ephemeral detail query

The heavy drawer body will be conditionally mounted only while the sheet is open or completing its close animation. After close:

- Reset navigation records to the current table page rather than retaining fetched adjacent pages.
- Clear the displayed detailed request reference.
- Clear generated Curl text and close the Curl dialog.
- Reset loading, expansion, and active-tab state where appropriate.
- Detach the detail-query observer by unmounting the body/query-owning component after the close animation.
- Configure only the quick-view detail query with a short or zero inactive `gcTime`, leaving full detail-page caching unchanged.

The sheet shell remains available for correct Radix open/close behavior. Large JSON trees and request/response payloads do not remain mounted while closed.

### 4. Linear live-preview chunk buffering

The SSE handler will append chunks to a mutable ref instead of cloning the full chunk array per event. React updates will expose immutable snapshots at a controlled cadence, such as one `requestAnimationFrame` batch, rather than once for every raw network event.

Behavior:

- Each received chunk is appended once to the ref.
- At most one pending UI flush is scheduled.
- A flush publishes a snapshot for rendering and parsing.
- Completion performs a final flush before the authoritative detail refetch.
- Cleanup cancels a pending animation frame, clears the ref, aborts the fetch, and clears reconnect timers.
- Replay skipping remains based on the total received chunk count.

This retains complete in-progress output while changing repeated array copying from per-chunk quadratic allocation to batched linear growth.

## Error and concurrency handling

- Adjacent-page requests retain the existing loading guard to prevent overlapping navigation fetches.
- Drawer state updates check the current open lifecycle before publishing asynchronously fetched data.
- Fetch failures continue to release the loading flag through `finally`; existing query and global error handling remain unchanged.
- SSE reconnect logic remains unchanged except that pending UI flushes are part of cleanup.
- State reset must not run during render; all lifecycle cleanup occurs in effects or event handlers.

## Testing

### Automated tests

Add focused unit tests for pure lifecycle helpers:

- Animation queue never exceeds `pageSize`.
- Queue deduplicates IDs and drops records absent from the latest page.
- Saturated queues retain the newest applicable records and converge to current server data.
- Drawer page merges deduplicate records.
- Older and newer page-deque merges preserve the selected request and produce the correct index.
- Drawer navigation retains at most three pages and preserves cursors for refetching evicted ranges.
- Live-preview chunk batching publishes all chunks in order and cancels pending work on cleanup where feasible with the existing test setup.

Run the existing frontend unit tests. Per repository rules, do not run frontend lint or build unless explicitly requested.

### Browser verification

Using the managed frontend server and an authenticated local account:

1. Open Requests and establish a browser memory baseline.
2. Enable auto-refresh under sustained request traffic; verify pending animation storage remains bounded and the table converges to current data.
3. Open the drawer, cross many page boundaries in both directions, and verify navigation remains correct.
4. Close and reopen the drawer repeatedly; verify request/response JSON is not retained by mounted drawer UI after close.
5. Open a processing streamed request; verify complete live text, reconnect behavior, and completion refetch.
6. Navigate away, force garbage collection when available, and confirm retained memory returns near baseline without growing across repetitions.
7. Check browser console and failed network requests.

## Acceptance criteria

- No client collection introduced by these paths grows without a defined bound.
- Requests, Traces, and Threads auto-refresh remain functional.
- Quick-view navigation works across server pages in both directions.
- Closing the quick-view drawer releases its heavy content and inactive detail query.
- Live preview displays every chunk in order and transitions correctly to completed request data.
- Added unit tests pass.
- Browser verification shows no repeated-session upward memory trend attributable to the repaired paths.
