# Requests Page Memory Leak Repair Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Bound all request-page lifecycle collections, release quick-view payloads after close, and remove per-chunk full-array copying from live preview without changing visible behavior.

**Architecture:** Add three import-free state helpers: one reconciles a bounded animated-list queue, one manages a three-page request-navigation deque, and one batches preview chunks per animation frame. React hooks/components own only lifecycle wiring; pure helpers carry the algorithms and are tested through the repository's Node/TypeScript test pattern.

**Tech Stack:** React 19, TypeScript 5.8, TanStack Query 5, TanStack Table 8, Radix Sheet, Node test runner.

## Global Constraints

- At most one sub-agent may run concurrently.
- Preserve row animation, cross-page quick-view navigation, Curl preview, and complete live-preview output.
- Do not truncate completed request/response content or channel filter options.
- Do not change global React Query cache defaults.
- The shared animated-list fix must remain compatible with Requests, Traces, and Threads.
- Use `pnpm`; do not run frontend lint or build unless the user explicitly asks.
- Browser verification is mandatory for frontend changes; the managed frontend server must not be restarted.
- Before every commit, run root and `llm` Go build, lint, and test commands from `AGENTS.md`.
- Clear the golangci-lint cache and run `--new-from-rev HEAD` before each commit; no `.exe` file may be staged.

## File Structure

- Create `frontend/src/hooks/animated-list-state.ts`: import-free queue reconciliation for auto-refresh lists.
- Create `frontend/src/hooks/animated-list-state.test.mjs`: queue bound, deduplication, and stale-entry tests.
- Modify `frontend/src/hooks/useAnimatedList.ts`: delegate pending-item reconciliation to the pure helper.
- Create `frontend/src/features/requests/components/request-navigation-state.ts`: import-free three-page deque and selected-index calculation.
- Create `frontend/src/features/requests/components/request-navigation-state.test.mjs`: older/newer merge, eviction, deduplication, and cursor preservation tests.
- Modify `frontend/src/features/requests/components/request-body-drawer.tsx`: use the page deque; split heavy detail content into an ephemeral child; invalidate asynchronous navigation updates after close.
- Modify `frontend/src/features/requests/data/requests.ts`: expose a per-useRequest `gcTime` option for the quick-view query only.
- Create `frontend/src/features/requests/components/preview-chunk-batcher.ts`: import-free animation-frame batcher.
- Create `frontend/src/features/requests/components/preview-chunk-batcher.test.mjs`: scheduling, ordering, explicit flush, and disposal tests.
- Modify `frontend/src/features/requests/components/request-detail-page.tsx`: append chunks once to a ref and publish only batched renders.

---

### Task 1: Bound the shared auto-refresh animation queue

**Files:**
- Create: `frontend/src/hooks/animated-list-state.ts`
- Create: `frontend/src/hooks/animated-list-state.test.mjs`
- Modify: `frontend/src/hooks/useAnimatedList.ts:1-90`

**Interfaces:**
- Consumes: list records shaped as `{ id: string; createdAt: Date | string }` and the current `pageSize`.
- Produces: `reconcileAnimatedQueue<T>(queue: T[], incoming: T[], displayed: T[], maxSize: number): T[]`.

- [ ] **Step 1: Write the failing queue tests**

Create `frontend/src/hooks/animated-list-state.test.mjs`:

```js
import assert from 'node:assert/strict';
import test from 'node:test';
import ts from 'typescript';
import { readFileSync } from 'node:fs';
import { join } from 'node:path';

const source = readFileSync(join(import.meta.dirname, 'animated-list-state.ts'), 'utf8');
const transpiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2023 },
}).outputText;
const { reconcileAnimatedQueue } = await import(
  `data:text/javascript;base64,${Buffer.from(transpiled).toString('base64')}`
);

const item = (id, second) => ({ id, createdAt: `2026-07-13T00:00:${String(second).padStart(2, '0')}Z` });

test('caps the pending animation queue to the configured page size', () => {
  const incoming = Array.from({ length: 6 }, (_, index) => item(String(index), index));
  const result = reconcileAnimatedQueue([], incoming, [item('0', 0)], 3);

  assert.deepEqual(result.map(({ id }) => id), ['3', '4', '5']);
});

test('deduplicates displayed and already queued records', () => {
  const result = reconcileAnimatedQueue(
    [item('2', 2)],
    [item('3', 3), item('2', 2), item('1', 1)],
    [item('1', 1)],
    10
  );

  assert.deepEqual(result.map(({ id }) => id), ['2', '3']);
});

test('drops queued records that are absent from the latest server page', () => {
  const result = reconcileAnimatedQueue(
    [item('stale', 2), item('kept', 3)],
    [item('new', 4), item('kept', 3)],
    [item('old', 1)],
    10
  );

  assert.deepEqual(result.map(({ id }) => id), ['kept', 'new']);
});
```

- [ ] **Step 2: Run the focused test and confirm the red state**

Run:

```powershell
pnpm --dir frontend exec node --test src/hooks/animated-list-state.test.mjs
```

Expected: FAIL with `ENOENT` for `animated-list-state.ts`.

- [ ] **Step 3: Implement the bounded reconciliation helper**

Create `frontend/src/hooks/animated-list-state.ts`:

```ts
export interface AnimatedListItem {
  id: string;
  createdAt: Date | string;
}

function getTimestamp(value: Date | string): number {
  return value instanceof Date ? value.getTime() : new Date(value).getTime();
}

export function reconcileAnimatedQueue<T extends AnimatedListItem>(
  queue: T[],
  incoming: T[],
  displayed: T[],
  maxSize: number
): T[] {
  if (maxSize <= 0) return [];

  const incomingIds = new Set(incoming.map(({ id }) => id));
  const displayedIds = new Set(displayed.map(({ id }) => id));
  const newestDisplayedTime = displayed.length > 0 ? getTimestamp(displayed[0].createdAt) : Number.NEGATIVE_INFINITY;
  const retained = queue.filter(({ id }) => incomingIds.has(id) && !displayedIds.has(id));
  const queuedIds = new Set(retained.map(({ id }) => id));
  const additions = incoming.filter(({ id, createdAt }) => {
    return !displayedIds.has(id) && !queuedIds.has(id) && getTimestamp(createdAt) > newestDisplayedTime;
  });
  const candidates = [...retained, ...additions]
    .sort((left, right) => getTimestamp(left.createdAt) - getTimestamp(right.createdAt));

  return candidates.length <= maxSize ? candidates : candidates.slice(-maxSize);
}
```

Modify `frontend/src/hooks/useAnimatedList.ts` to import the helper and replace the `sortedNewItems.forEach(...)` block with:

```ts
queueRef.current = reconcileAnimatedQueue(queueRef.current, data, currentDisplayed, pageSize);
```

Remove the now-unused `newItems`, `sortedNewItems`, and duplicate enqueue code. Add `pageSize` to the reconciliation effect dependency list.

- [ ] **Step 4: Run the focused and complete frontend unit suites**

Run:

```powershell
pnpm --dir frontend exec node --test src/hooks/animated-list-state.test.mjs
pnpm --dir frontend test:unit
```

Expected: all tests PASS; no queue result exceeds `pageSize`.

- [ ] **Step 5: Run required pre-commit verification**

Run:

```powershell
golangci-lint cache clean
go build ./...
Push-Location llm; try { go build ./... } finally { Pop-Location }
golangci-lint run --timeout 10m --max-same-issues 50 ./...
Push-Location llm; try { golangci-lint run --timeout 10m --max-same-issues 50 ./... } finally { Pop-Location }
go test ./...
Push-Location llm; try { go test ./... } finally { Pop-Location }
golangci-lint run --timeout 10m --max-same-issues 50 --new-from-rev HEAD ./...
Push-Location llm; try { golangci-lint run --timeout 10m --max-same-issues 50 --new-from-rev HEAD ./... } finally { Pop-Location }
git diff --check
git status --short
```

Expected: both modules build and test; both lint runs report `0 issues`; no `.exe` is staged.

- [ ] **Step 6: Commit the bounded queue**

```powershell
git add frontend/src/hooks/animated-list-state.ts frontend/src/hooks/animated-list-state.test.mjs frontend/src/hooks/useAnimatedList.ts
git commit -m "fix(frontend): bound auto-refresh animation queue"
```

Expected: one commit containing only the queue helper, tests, and hook integration.

---

### Task 2: Bound drawer navigation and release closed detail payloads

**Files:**
- Create: `frontend/src/features/requests/components/request-navigation-state.ts`
- Create: `frontend/src/features/requests/components/request-navigation-state.test.mjs`
- Modify: `frontend/src/features/requests/components/request-body-drawer.tsx:34-423`
- Modify: `frontend/src/features/requests/data/requests.ts:387-459`

**Interfaces:**
- Consumes: `Request[]`, `RequestConnection['pageInfo']`, navigation direction, and a page limit of `3`.
- Produces: `NavigationState<T, P>`, `createNavigationState`, `flattenNavigationPages`, and `mergeNavigationPage`.
- Produces: optional `gcTime?: number` in `useRequest` options; existing callers retain default TanStack Query behavior.

- [ ] **Step 1: Write failing page-deque tests**

Create `frontend/src/features/requests/components/request-navigation-state.test.mjs` with the same TypeScript data-URL loader used in Task 1, targeting `request-navigation-state.ts`, then add:

```js
const item = (id) => ({ id });
const pageInfo = (start, end, hasPreviousPage = true, hasNextPage = true) => ({
  startCursor: start,
  endCursor: end,
  hasPreviousPage,
  hasNextPage,
});
const page = (ids, start, end) => ({ items: ids.map(item), pageInfo: pageInfo(start, end) });

test('appends an older page and selects its first item', () => {
  const initial = createNavigationState(page(['5', '4'], 's5', 'e4'), 0);
  const result = mergeNavigationPage(initial, page(['3', '2'], 's3', 'e2'), 'older', 3);

  assert.deepEqual(flattenNavigationPages(result.pages).map(({ id }) => id), ['5', '4', '3', '2']);
  assert.equal(result.currentIndex, 2);
});

test('prepends a newer page and selects its last item', () => {
  const initial = createNavigationState(page(['3', '2'], 's3', 'e2'), 0);
  const result = mergeNavigationPage(initial, page(['5', '4'], 's5', 'e4'), 'newer', 3);

  assert.deepEqual(flattenNavigationPages(result.pages).map(({ id }) => id), ['5', '4', '3', '2']);
  assert.equal(result.currentIndex, 1);
});

test('deduplicates overlapping request IDs', () => {
  const initial = createNavigationState(page(['5', '4'], 's5', 'e4'), 0);
  const result = mergeNavigationPage(initial, page(['4', '3'], 's4', 'e3'), 'older', 3);

  assert.deepEqual(flattenNavigationPages(result.pages).map(({ id }) => id), ['5', '4', '3']);
});

test('retains at most three pages and preserves the retained boundary cursor', () => {
  let state = createNavigationState(page(['8'], 's8', 'e8'), 0);
  state = mergeNavigationPage(state, page(['7'], 's7', 'e7'), 'older', 3);
  state = mergeNavigationPage(state, page(['6'], 's6', 'e6'), 'older', 3);
  state = mergeNavigationPage(state, page(['5'], 's5', 'e5'), 'older', 3);

  assert.equal(state.pages.length, 3);
  assert.deepEqual(flattenNavigationPages(state.pages).map(({ id }) => id), ['7', '6', '5']);
  assert.equal(state.pages[0].pageInfo.startCursor, 's7');
  assert.equal(state.pages[0].pageInfo.hasPreviousPage, true);
  assert.equal(state.pages.at(-1).pageInfo.endCursor, 'e5');
});
```

- [ ] **Step 2: Run the focused test and confirm the red state**

Run:

```powershell
pnpm --dir frontend exec node --test src/features/requests/components/request-navigation-state.test.mjs
```

Expected: FAIL with `ENOENT` for `request-navigation-state.ts`.

- [ ] **Step 3: Implement the generic page deque**

Create `frontend/src/features/requests/components/request-navigation-state.ts`:

```ts
export interface RequestNavigationPageInfo {
  hasPreviousPage: boolean;
  hasNextPage: boolean;
  startCursor?: string | null;
  endCursor?: string | null;
}

export interface NavigationPage<T, P extends RequestNavigationPageInfo> {
  items: T[];
  pageInfo: P;
}

export interface NavigationState<T, P extends RequestNavigationPageInfo> {
  pages: NavigationPage<T, P>[];
  currentIndex: number;
}

export type NavigationDirection = 'older' | 'newer';

export function flattenNavigationPages<T, P extends RequestNavigationPageInfo>(pages: NavigationPage<T, P>[]): T[] {
  return pages.flatMap(({ items }) => items);
}

export function createNavigationState<T, P extends RequestNavigationPageInfo>(
  page: NavigationPage<T, P>,
  currentIndex: number
): NavigationState<T, P> {
  return { pages: page.items.length > 0 ? [page] : [], currentIndex };
}

export function mergeNavigationPage<T extends { id: string }, P extends RequestNavigationPageInfo>(
  state: NavigationState<T, P>,
  incomingPage: NavigationPage<T, P>,
  direction: NavigationDirection,
  maxPages: number
): NavigationState<T, P> {
  const targetId = direction === 'older'
    ? incomingPage.items[0]?.id
    : incomingPage.items.at(-1)?.id;
  const orderedPages = direction === 'older'
    ? [...state.pages, incomingPage]
    : [incomingPage, ...state.pages];
  const seen = new Set<string>();
  const deduplicatedPages = orderedPages
    .map((page) => ({
      ...page,
      items: page.items.filter(({ id }) => {
        if (seen.has(id)) return false;
        seen.add(id);
        return true;
      }),
    }))
    .filter(({ items }) => items.length > 0);
  const evictedNewerPage = direction === 'older' && deduplicatedPages.length > maxPages;
  const evictedOlderPage = direction === 'newer' && deduplicatedPages.length > maxPages;
  const pages = direction === 'older'
    ? deduplicatedPages.slice(-maxPages)
    : deduplicatedPages.slice(0, maxPages);

  if (evictedNewerPage && pages[0]) {
    pages[0] = {
      ...pages[0],
      pageInfo: { ...pages[0].pageInfo, hasPreviousPage: true },
    };
  }
  if (evictedOlderPage && pages.at(-1)) {
    const lastIndex = pages.length - 1;
    pages[lastIndex] = {
      ...pages[lastIndex],
      pageInfo: { ...pages[lastIndex].pageInfo, hasNextPage: true },
    };
  }

  const items = flattenNavigationPages(pages);
  const targetIndex = targetId ? items.findIndex(({ id }) => id === targetId) : -1;

  return { pages, currentIndex: targetIndex >= 0 ? targetIndex : state.currentIndex };
}
```

- [ ] **Step 4: Replace the drawer's unbounded list with the page deque**

In `request-body-drawer.tsx`:

1. Add `MAX_NAVIGATION_PAGES = 3`.
2. Replace `allRequests`, `navPageInfo`, and `currentIndex` with one `NavigationState<Request, RequestConnection['pageInfo']>` state. Normalize a missing `initialPageInfo` before calling `createNavigationState` by using `{ hasPreviousPage: false, hasNextPage: false }`.
3. Build visible records with `flattenNavigationPages`.
4. Read newer-page metadata from the first retained page and older-page metadata from the last retained page. When the newest page is evicted after an older fetch, set the retained first page's `hasPreviousPage` to `true`; when the oldest page is evicted after a newer fetch, set the retained last page's `hasNextPage` to `true`. This preserves the ability to refetch an evicted range.
5. On adjacent fetch completion, call `mergeNavigationPage` and use its `currentIndex`.
6. Increment a `navigationGenerationRef` whenever the drawer opens or closes; after `await fetchAdjacentRequestPage`, discard the result if the captured generation no longer matches.
7. On close, set navigation state to `{ pages: [], currentIndex: 0 }` and reset `isLoadingMore`.

Use this lifecycle pattern:

```ts
const navigationGenerationRef = useRef(0);

useEffect(() => {
  const justOpened = open && !prevOpenRef.current;
  prevOpenRef.current = open;

  if (justOpened) {
    navigationGenerationRef.current += 1;
    setNavigation(createNavigationState(
      { items: initialRequests, pageInfo: initialPageInfo ?? { hasPreviousPage: false, hasNextPage: false } },
      initialIndex
    ));
    return;
  }

  if (!open) {
    navigationGenerationRef.current += 1;
    setNavigation({ pages: [], currentIndex: 0 });
    setIsLoadingMore(false);
  }
}, [open, initialRequests, initialPageInfo, initialIndex]);
```

Capture `const generation = navigationGenerationRef.current` before each fetch. If the generation no longer matches after the fetch, return without merging. In `finally`, clear the loading flag only when the captured generation still matches; the close effect owns resetting it for stale generations.

- [ ] **Step 5: Make the heavy detail subtree ephemeral**

Extract the detail query, `displayedRequestRef`, tab/expand state, Curl state, JSON trees, and `CurlPreviewDialog` into a local `RequestBodyDrawerContent` child. Mount it only when `open && canRenderBody && currentRequestId`.

The child must call:

```ts
const { data: request, isLoading, isFetching } = useRequest(currentRequestId, {
  projectId,
  enabled: true,
  includeAdminFields,
  gcTime: 0,
});
```

When the child unmounts on close, its query observer, large displayed request reference, generated Curl command, and JSON tree state are released together. Keep the Sheet shell and navigation header in the parent so the Radix close animation remains intact.

- [ ] **Step 6: Add the narrow query cache option**

In `frontend/src/features/requests/data/requests.ts`, add this field to the `useRequest` options type:

```ts
gcTime?: number;
```

Pass it to `useQuery`:

```ts
gcTime: options?.gcTime,
```

Do not add `gcTime` to the query key and do not change `QueryClient` defaults. Only the quick-view child passes `0`.

- [ ] **Step 7: Run focused and complete frontend unit suites**

Run:

```powershell
pnpm --dir frontend exec node --test src/features/requests/components/request-navigation-state.test.mjs
pnpm --dir frontend test:unit
```

Expected: all tests PASS; navigation state retains no more than three pages.

- [ ] **Step 8: Browser-check drawer navigation before commit**

Using the existing managed frontend server:

1. Open `/project/requests` or `/admin/requests` while authenticated.
2. Open quick view and move across at least four server page boundaries in the older direction.
3. Move back across an evicted boundary in the newer direction and confirm data refetches without skipped or duplicated requests.
4. Close and reopen the drawer; confirm the clicked request opens and no stale Curl dialog, tab, or expanded JSON state remains.
5. Check the browser console and request list for errors.

Expected: navigation remains correct; closed drawer content is absent from the accessibility snapshot/DOM; no failed adjacent-page fetch appears.

- [ ] **Step 9: Run required pre-commit verification and commit**

Run the complete pre-commit command block from Task 1 Step 5, then:

```powershell
git add frontend/src/features/requests/components/request-navigation-state.ts frontend/src/features/requests/components/request-navigation-state.test.mjs frontend/src/features/requests/components/request-body-drawer.tsx frontend/src/features/requests/data/requests.ts
git commit -m "fix(requests): bound quick-view memory"
```

Expected: all required checks pass and the commit contains only drawer lifecycle changes and tests.

---

### Task 3: Remove per-event full-array copying from live preview

**Files:**
- Create: `frontend/src/features/requests/components/preview-chunk-batcher.ts`
- Create: `frontend/src/features/requests/components/preview-chunk-batcher.test.mjs`
- Modify: `frontend/src/features/requests/components/request-detail-page.tsx:118-341`

**Interfaces:**
- Consumes: individual parsed preview chunks and browser frame scheduling functions.
- Produces: `createPreviewChunkBatcher<T>(publish, schedule?, cancel?): { push; flush; dispose }`.

- [ ] **Step 1: Write failing batcher tests**

Create `preview-chunk-batcher.test.mjs` with the TypeScript data-URL loader and these tests:

```js
test('publishes queued chunks in order with one scheduled frame', () => {
  let scheduled;
  let scheduleCount = 0;
  const published = [];
  const batcher = createPreviewChunkBatcher(
    (batch) => published.push(batch),
    (callback) => { scheduleCount += 1; scheduled = callback; return 1; },
    () => {}
  );

  batcher.push('a');
  batcher.push('b');
  batcher.push('c');

  assert.equal(scheduleCount, 1);
  scheduled();
  assert.deepEqual(published, [['a', 'b', 'c']]);
});

test('flush publishes immediately and cancels the pending frame', () => {
  const published = [];
  const canceled = [];
  const batcher = createPreviewChunkBatcher(
    (batch) => published.push(batch),
    () => 7,
    (id) => canceled.push(id)
  );

  batcher.push('a');
  batcher.flush();

  assert.deepEqual(canceled, [7]);
  assert.deepEqual(published, [['a']]);
});

test('dispose cancels pending work and drops unpublished chunks', () => {
  let scheduled;
  const published = [];
  const batcher = createPreviewChunkBatcher(
    (batch) => published.push(batch),
    (callback) => { scheduled = callback; return 9; },
    () => {}
  );

  batcher.push('a');
  batcher.dispose();
  scheduled();

  assert.deepEqual(published, []);
});
```

- [ ] **Step 2: Run the focused test and confirm the red state**

Run:

```powershell
pnpm --dir frontend exec node --test src/features/requests/components/preview-chunk-batcher.test.mjs
```

Expected: FAIL with `ENOENT` for `preview-chunk-batcher.ts`.

- [ ] **Step 3: Implement the frame batcher**

Create `preview-chunk-batcher.ts`:

```ts
type ScheduleFrame = (callback: () => void) => number;
type CancelFrame = (id: number) => void;

export function createPreviewChunkBatcher<T>(
  publish: (batch: T[]) => void,
  schedule: ScheduleFrame = requestAnimationFrame,
  cancel: CancelFrame = cancelAnimationFrame
) {
  let pending: T[] = [];
  let frameId: number | null = null;
  let disposed = false;

  const publishPending = () => {
    frameId = null;
    if (disposed || pending.length === 0) return;
    const batch = pending;
    pending = [];
    publish(batch);
  };

  return {
    push(item: T) {
      if (disposed) return;
      pending.push(item);
      if (frameId === null) frameId = schedule(publishPending);
    },
    flush() {
      if (frameId !== null) cancel(frameId);
      publishPending();
    },
    dispose() {
      disposed = true;
      pending = [];
      if (frameId !== null) cancel(frameId);
      frameId = null;
    },
  };
}
```

- [ ] **Step 4: Wire the batcher into the SSE lifecycle**

In `request-detail-page.tsx`:

1. Add `previewChunksRef = useRef<any[]>([])` beside the existing preview refs.
2. Reset that ref whenever preview state is reset or a new stream starts.
3. Inside the streaming effect, create one batcher whose publisher appends once to the mutable ref and publishes a new request wrapper without cloning prior chunks:

```ts
const chunkBatcher = createPreviewChunkBatcher<any>((batch) => {
  if (isDisposed) return;
  previewChunksRef.current.push(...batch);
  setPreviewRequest((currentRequest) => currentRequest
    ? { ...currentRequest, responseChunks: previewChunksRef.current }
    : currentRequest
  );
});
```

4. Replace the current `responseChunks: [...oldChunks, nextChunk]` state update with `chunkBatcher.push(nextChunk)`.
5. On `preview.completed`, call `chunkBatcher.flush()` before the authoritative refetch.
6. On static fallback, dispose the batcher, replace `previewChunksRef.current` with the fallback chunks, and publish that array once.
7. In effect cleanup, call `chunkBatcher.dispose()` before clearing `previewChunksRef.current`, then abort and clear reconnect state as today.

The retained raw data remains linear in the actual response size; the implementation no longer allocates and copies the complete prior history for every SSE event.

- [ ] **Step 5: Run focused and complete frontend unit suites**

Run:

```powershell
pnpm --dir frontend exec node --test src/features/requests/components/preview-chunk-batcher.test.mjs
pnpm --dir frontend test:unit
```

Expected: all tests PASS; batch order and disposal behavior match the tests.

- [ ] **Step 6: Browser-check live preview and cleanup**

Using a processing streamed request with live preview enabled:

1. Open the project request detail route.
2. Confirm content, reasoning, and tool-call deltas appear in order.
3. Confirm only one reconnect loop exists after a transient disconnect.
4. Let the request complete and confirm the full request detail replaces the preview.
5. Navigate away while a stream is active and confirm the preview request is aborted and no further UI updates or console warnings occur.

Expected: complete ordered output, correct completion refetch, and no post-unmount updates.

- [ ] **Step 7: Run required pre-commit verification and commit**

Run the complete pre-commit command block from Task 1 Step 5, then:

```powershell
git add frontend/src/features/requests/components/preview-chunk-batcher.ts frontend/src/features/requests/components/preview-chunk-batcher.test.mjs frontend/src/features/requests/components/request-detail-page.tsx
git commit -m "fix(requests): batch live preview chunks"
```

Expected: all required checks pass and only preview batching files are committed.

---

### Task 4: Verify bounded memory across the complete Requests workflow

**Files:**
- Verify only; do not create or commit heap snapshots.

**Interfaces:**
- Consumes: the three completed implementation commits.
- Produces: browser and command evidence that memory growth is bounded and no regression remains.

- [ ] **Step 1: Run the complete frontend unit suite once more**

Run:

```powershell
pnpm --dir frontend test:unit
```

Expected: all existing and newly added tests PASS.

- [ ] **Step 2: Verify auto-refresh on all shared-hook consumers**

In the browser, enable auto-refresh on Requests, Traces, and Threads one page at a time. For Requests, use page size 50 and sustained incoming traffic if available.

Expected:

- Tables continue animating new rows.
- No table displays more than its selected page size.
- Console contains no update-depth, stale-state, or unmounted-update errors.
- After several refreshes, each table converges to the latest server page.

- [ ] **Step 3: Measure repeated Requests workflows**

Use DevTools Memory or `performance.memory.usedJSHeapSize` where available:

1. Record a baseline after Requests becomes idle.
2. Repeat 10 times: open quick view, navigate several records, close quick view.
3. Navigate across at least five server page boundaries and back.
4. Navigate away from Requests, take a heap snapshot to trigger collection, then return.
5. Repeat the cycle once and compare the post-collection retained heap.

Expected: the second cycle does not retain another copy of every traversed page or drawer payload. Temporary allocations may rise while JSON is rendered, but post-close/post-navigation memory does not show a repeated-session upward trend from `RequestBodyDrawer`, `queueRef`, or inactive quick-view request data.

- [ ] **Step 4: Inspect console and network failures**

Check preserved console messages and network requests covering the verification cycle.

Expected: no new errors, no runaway adjacent-page requests, no duplicate preview streams, and no requests continuing after the relevant UI is unmounted.

- [ ] **Step 5: Confirm repository state and commit history**

Run:

```powershell
git status --short
git log -4 --oneline
```

Expected: clean working tree; design plus three focused fix commits are visible; no `.exe`, heap snapshot, log, or browser artifact is tracked.

## Plan Self-Review

- Spec coverage: Task 1 bounds shared auto-refresh state; Task 2 bounds and clears drawer state plus quick-view query retention; Task 3 removes per-event full-array copying while preserving all chunks; Task 4 covers Requests, Traces, Threads, drawer, stream, console, network, and repeated memory measurements.
- Placeholder scan: no `TBD`, deferred implementation, unspecified test, or unnamed error-handling step remains.
- Type consistency: `NavigationPage.items`, `NavigationState.pages/currentIndex`, `gcTime`, and `createPreviewChunkBatcher.push/flush/dispose` use identical names in producer and consumer tasks.
