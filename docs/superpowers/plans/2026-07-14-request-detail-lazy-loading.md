# Request Detail Lazy Loading Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make request detail routes load metadata first and fetch main/execution payloads only while the user explicitly views them.

**Architecture:** Split the current broad GraphQL operations and shared React Query entry into metadata, main request content, main response content, execution summaries, and one-execution content. The shared detail page owns the active tab and live-preview eligibility; focused child panels own content queries with immediate cache collection.

**Tech Stack:** React 19, TypeScript 5.8, TanStack Query 5, gqlgen GraphQL, Zod, Radix Tabs/Collapsible, Node test runner, Chrome DevTools, Go pprof.

## Global Constraints

- Project routes must pass `X-Project-ID`; administrator routes must remain system-scoped and must not inherit a selected project.
- Large content stays in browser memory only while its owning view is active; do not persist it in localStorage or add a server result cache.
- The Overview tab is the default and performs no main/execution content query.
- Main request content, main response content, and one execution content use `gcTime: 0` and isolated query keys.
- At most one execution content panel is expanded at a time.
- Live preview runs only while the Response tab is active and clears all browser buffers on deactivation.
- Preserve copy, JSON download, cURL, response preview, chunk dialog, audio/video, project/admin authorization, and existing quick-view behavior.
- Do not run frontend lint/build unless explicitly requested by the user; run focused unit tests and browser/runtime verification.

---

## File Structure

- Modify `frontend/src/features/requests/data/request-query-key.ts` — define isolated metadata/request-content/response-content/execution-content key builders.
- Modify `frontend/src/features/requests/data/request-query-key.test.mjs` — prove cache-key isolation.
- Modify `frontend/src/features/requests/data/schema.ts` — introduce explicit metadata and content schemas while retaining list/quick-view compatibility.
- Modify `frontend/src/features/requests/data/requests.ts` — define split GraphQL operations and hooks with enable/gc/cancellation behavior.
- Create `frontend/src/features/requests/data/request-detail-query.test.mjs` — statically prove GraphQL field boundaries and hook lifecycle options.
- Modify `frontend/src/features/requests/components/request-detail-page.tsx` — own the project detail active tab and gate live preview by Response visibility.
- Modify `frontend/src/features/requests/components/request-detail-global-page.tsx` — own the admin detail active tab and fetch metadata once.
- Modify `frontend/src/features/requests/components/request-detail-content.tsx` — render Overview by default, fetch each payload only in its panel, and expand only one execution.
- Create `frontend/src/features/requests/components/request-execution-content.tsx` — focused one-execution payload panel with local loading/error/actions.
- Create `frontend/src/features/requests/components/request-content-state.ts` — pure active-tab and single-expansion state helpers for deterministic testing.
- Create `frontend/src/features/requests/components/request-content-state.test.mjs` — cover default Overview, tab activation, and single execution expansion.
- Modify `frontend/src/locales/en/requests.json` and `frontend/src/locales/zh-CN/requests.json` — add Overview, load-error, retry, and execution expand/collapse labels.

---

### Task 1: Isolate Detail Cache Keys

**Files:**
- Modify: `frontend/src/features/requests/data/request-query-key.ts`
- Modify: `frontend/src/features/requests/data/request-query-key.test.mjs`

**Interfaces:**
- Consumes: existing `id`, permission shape, `projectId`, and `includeAdminFields` scope inputs.
- Produces:
  - `buildRequestMetadataQueryKey(input)`
  - `buildRequestContentQueryKey({...input, content: 'request' | 'response'})`
  - `buildRequestExecutionContentQueryKey({...input, executionId})`
  - existing `buildRequestQueryKey(input)` unchanged for quick-view compatibility.

- [ ] **Step 1: Extend the key test with failing isolation assertions**

```js
test('isolates metadata and each main content kind', () => {
  const metadata = buildRequestMetadataQueryKey(params);
  const requestContent = buildRequestContentQueryKey({ ...params, content: 'request' });
  const responseContent = buildRequestContentQueryKey({ ...params, content: 'response' });

  assert.notDeepEqual(metadata, requestContent);
  assert.notDeepEqual(requestContent, responseContent);
  assert.deepEqual(metadata.slice(0, 2), ['request', 'metadata']);
  assert.deepEqual(requestContent.slice(0, 3), ['request', 'content', 'request']);
});

test('isolates execution content by execution id and scope', () => {
  const first = buildRequestExecutionContentQueryKey({ ...params, executionId: 'RequestExecution:1' });
  const second = buildRequestExecutionContentQueryKey({ ...params, executionId: 'RequestExecution:2' });
  assert.notDeepEqual(first, second);
  assert.deepEqual(first.slice(0, 2), ['request-execution', 'content']);
});
```

- [ ] **Step 2: Run the focused test and confirm it fails on missing exports**

Run: `cd frontend && node --test src/features/requests/data/request-query-key.test.mjs`

Expected: FAIL because the three new builders are not exported.

- [ ] **Step 3: Implement the key builders**

```ts
interface RequestScopedQueryKeyInput {
  id: string;
  permissions: unknown;
  projectId?: string | null;
  includeAdminFields?: boolean;
}

export function buildRequestMetadataQueryKey(input: RequestScopedQueryKeyInput) {
  return ['request', 'metadata', input.id, input.permissions, input.projectId, input.includeAdminFields] as const;
}

export function buildRequestContentQueryKey(
  input: RequestScopedQueryKeyInput & { content: 'request' | 'response' }
) {
  return ['request', 'content', input.content, input.id, input.permissions, input.projectId, input.includeAdminFields] as const;
}

export function buildRequestExecutionContentQueryKey(
  input: RequestScopedQueryKeyInput & { executionId: string }
) {
  return ['request-execution', 'content', input.executionId, input.id, input.permissions, input.projectId, input.includeAdminFields] as const;
}
```

Keep the existing `buildRequestQueryKey` implementation for the quick-view drawer.

- [ ] **Step 4: Run the focused test and confirm it passes**

Run: `cd frontend && node --test src/features/requests/data/request-query-key.test.mjs`

Expected: all key tests PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/features/requests/data/request-query-key.ts frontend/src/features/requests/data/request-query-key.test.mjs
git commit -m "test(requests): isolate detail content cache keys"
```

### Task 2: Split GraphQL Operations and Hooks

**Files:**
- Modify: `frontend/src/features/requests/data/schema.ts`
- Modify: `frontend/src/features/requests/data/requests.ts`
- Create: `frontend/src/features/requests/data/request-detail-query.test.mjs`

**Interfaces:**
- Consumes: Task 1 key builders, `graphqlRequest`, permission-dependent fields, project headers, existing request/execution Zod fields.
- Produces:
  - `useRequestMetadata(id, options): UseQueryResult<RequestMetadata>`
  - `useRequestContent(id, {kind, enabled, projectId, includeAdminFields}): UseQueryResult<RequestContent>`
  - `useRequestExecutions(requestID, variables, {projectId, enabled})`: summary-only connection
  - `useRequestExecutionContent(requestID, executionID, options): UseQueryResult<RequestExecutionContent>`
  - existing `useRequest` retained for quick-view compatibility.

- [ ] **Step 1: Write a failing static boundary test**

Create `request-detail-query.test.mjs` that reads `requests.ts`, extracts exported query-builder function bodies by name, and asserts:

```js
test('metadata query omits request and response payloads', () => {
  const query = extractFunction('buildRequestMetadataQuery');
  for (const field of ['requestHeaders', 'requestBody', 'responseBody', 'responseChunks']) {
    assert.doesNotMatch(query, new RegExp(`\\b${field}\\b`));
  }
});

test('request and response content queries select only their own payloads', () => {
  const requestQuery = extractFunction('buildRequestContentQuery');
  assert.match(requestQuery, /requestHeaders/);
  assert.match(requestQuery, /requestBody/);
  assert.doesNotMatch(requestQuery, /responseBody|responseChunks/);

  const responseQuery = extractFunction('buildResponseContentQuery');
  assert.match(responseQuery, /responseBody/);
  assert.match(responseQuery, /responseChunks/);
  assert.doesNotMatch(responseQuery, /requestHeaders|requestBody/);
});

test('execution summary query omits payloads and content query is node-scoped', () => {
  const summary = extractFunction('buildRequestExecutionSummariesQuery');
  assert.doesNotMatch(summary, /requestHeaders|requestBody|responseBody|responseChunks/);
  const content = extractFunction('buildRequestExecutionContentQuery');
  assert.match(content, /query GetRequestExecutionContent\(\$id: ID!\)/);
  assert.match(content, /node\(id: \$id\)/);
});

test('content hooks use immediate garbage collection', () => {
  assert.match(source, /useRequestContent[\s\S]*?gcTime:\s*0/);
  assert.match(source, /useRequestExecutionContent[\s\S]*?gcTime:\s*0/);
});
```

The extractor must count braces rather than use a single greedy regular expression, so nested template expressions do not invalidate the test.

- [ ] **Step 2: Run the test and verify it fails**

Run: `cd frontend && node --test src/features/requests/data/request-detail-query.test.mjs`

Expected: FAIL because split builders/hooks do not exist.

- [ ] **Step 3: Add explicit schemas/types**

In `schema.ts`, derive focused schemas from the existing shapes:

```ts
export const requestMetadataSchema = requestSchema.omit({
  requestHeaders: true,
  requestBody: true,
  responseBody: true,
  responseChunks: true,
  executions: true,
});
export type RequestMetadata = z.infer<typeof requestMetadataSchema>;

export const requestContentSchema = z.object({
  id: z.string(),
  requestHeaders: z.any().nullable().optional(),
  requestBody: z.any().nullable().optional(),
  responseBody: z.any().nullable().optional(),
  responseChunks: z.array(z.any()).nullable().optional(),
});
export type RequestContent = z.infer<typeof requestContentSchema>;

export const requestExecutionSummarySchema = requestExecutionSchema.omit({
  requestHeaders: true,
  requestBody: true,
  responseBody: true,
  responseChunks: true,
});
export type RequestExecutionSummary = z.infer<typeof requestExecutionSummarySchema>;

export const requestExecutionContentSchema = requestExecutionSchema.pick({
  id: true,
  channel: true,
  format: true,
  requestURL: true,
  requestHeaders: true,
  requestBody: true,
  responseBody: true,
  responseChunks: true,
});
export type RequestExecutionContent = z.infer<typeof requestExecutionContentSchema>;
```

Add a summary connection schema using `requestExecutionSummarySchema`.

- [ ] **Step 4: Implement the split query builders**

Add builders in `requests.ts`:

```ts
function buildRequestMetadataQuery(permissions, options = {}) { /* existing metadata fields, no payloads */ }
function buildRequestContentQuery() { return `query GetRequestContent($id: ID!) { node(id: $id) { ... on Request { id requestHeaders requestBody } } }`; }
function buildResponseContentQuery() { return `query GetResponseContent($id: ID!) { node(id: $id) { ... on Request { id responseBody responseChunks } } }`; }
function buildRequestExecutionSummariesQuery(permissions) { /* existing execution metadata, no payloads */ }
function buildRequestExecutionContentQuery(permissions) { /* node(id), one RequestExecution payload */ }
```

Metadata must preserve the current permitted project/API-key/channel/user fields and usage summary. Execution content must preserve channel `baseURL/type` and `requestURL/format` for cURL generation.

- [ ] **Step 5: Implement focused hooks**

Use the Task 1 keys, project headers, and `AbortSignal` supplied by TanStack Query:

```ts
queryFn: async ({ signal }) => graphqlRequest(query, variables, headers, signal)
```

If `graphqlRequest` does not yet accept `signal`, add an optional final `signal?: AbortSignal` parameter and pass it to `fetch`. Do not alter existing callers.

`useRequestMetadata` retains lightweight status polling. `useRequestContent` switches query by `kind`, uses `enabled`, and sets `gcTime: 0`. `useRequestExecutions` accepts `enabled` and parses the summary connection. `useRequestExecutionContent` uses one execution ID, `enabled`, and `gcTime: 0`.

- [ ] **Step 6: Run focused data tests**

Run: `cd frontend && node --test src/features/requests/data/request-query-key.test.mjs src/features/requests/data/request-detail-query.test.mjs`

Expected: all tests PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/features/requests/data/schema.ts frontend/src/features/requests/data/requests.ts frontend/src/features/requests/data/request-detail-query.test.mjs frontend/src/gql/graphql.ts
git commit -m "feat(requests): split detail payload queries"
```

### Task 3: Add Deterministic Detail View State

**Files:**
- Create: `frontend/src/features/requests/components/request-content-state.ts`
- Create: `frontend/src/features/requests/components/request-content-state.test.mjs`

**Interfaces:**
- Produces:
  - `type RequestDetailTab = 'overview' | 'request' | 'response' | 'executions'`
  - `DEFAULT_REQUEST_DETAIL_TAB = 'overview'`
  - `nextExpandedExecution(currentId, clickedId): string | null`

- [ ] **Step 1: Write failing state tests**

```js
test('defaults request details to overview', () => {
  assert.equal(DEFAULT_REQUEST_DETAIL_TAB, 'overview');
});

test('allows only one expanded execution and toggles the current one closed', () => {
  assert.equal(nextExpandedExecution(null, '1'), '1');
  assert.equal(nextExpandedExecution('1', '2'), '2');
  assert.equal(nextExpandedExecution('2', '2'), null);
});
```

- [ ] **Step 2: Run and verify failure**

Run: `cd frontend && node --test src/features/requests/components/request-content-state.test.mjs`

Expected: FAIL because the module does not exist.

- [ ] **Step 3: Implement the pure state helpers**

```ts
export type RequestDetailTab = 'overview' | 'request' | 'response' | 'executions';
export const DEFAULT_REQUEST_DETAIL_TAB: RequestDetailTab = 'overview';
export function nextExpandedExecution(currentId: string | null, clickedId: string): string | null {
  return currentId === clickedId ? null : clickedId;
}
```

- [ ] **Step 4: Run and verify pass**

Run: `cd frontend && node --test src/features/requests/components/request-content-state.test.mjs`

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/features/requests/components/request-content-state.ts frontend/src/features/requests/components/request-content-state.test.mjs
git commit -m "test(requests): define lazy detail view state"
```

### Task 4: Refactor Main Detail Tabs to Lazy Content

**Files:**
- Modify: `frontend/src/features/requests/components/request-detail-page.tsx`
- Modify: `frontend/src/features/requests/components/request-detail-global-page.tsx`
- Modify: `frontend/src/features/requests/components/request-detail-content.tsx`
- Modify: `frontend/src/locales/en/requests.json`
- Modify: `frontend/src/locales/zh-CN/requests.json`

**Interfaces:**
- Consumes: `useRequestMetadata`, `useRequestContent`, `RequestDetailTab`, `DEFAULT_REQUEST_DETAIL_TAB`.
- Produces: `RequestDetailContent` props include `request: RequestMetadata | undefined`, `activeTab`, `onActiveTabChange`, and live-preview props; it no longer calls the broad `useRequest` internally.

- [ ] **Step 1: Add translation keys in both locales**

English:

```json
"requests.detail.tabs.overview": "Overview",
"requests.detail.loadFailed": "Failed to load this request content.",
"requests.detail.retry": "Retry"
```

Chinese:

```json
"requests.detail.tabs.overview": "概览",
"requests.detail.loadFailed": "加载此请求内容失败。",
"requests.detail.retry": "重试"
```

- [ ] **Step 2: Fetch metadata once in each route page**

Replace broad route-page `useRequest` calls with `useRequestMetadata`. Add:

```ts
const [activeTab, setActiveTab] = useState<RequestDetailTab>(DEFAULT_REQUEST_DETAIL_TAB);
```

Pass the same metadata object to the route header and `RequestDetailContent`. For project live preview, compute:

```ts
const isResponseActive = activeTab === 'response';
```

and require `isResponseActive` before opening preview.

- [ ] **Step 3: Gate live preview lifecycle by Response visibility**

In `request-detail-page.tsx`, include `isResponseActive` in the effect gate and dependencies. When false, execute the same cleanup/reset path used when live preview is disabled: clear preview request state, streaming flags, reconnect state, counters, and `previewChunksRef.current`.

On preview completion, refetch metadata only. Persisted response content will be loaded by the active Response panel under its own key.

- [ ] **Step 4: Convert the shared tabs to four controlled tabs**

```tsx
<Tabs value={activeTab} onValueChange={(value) => onActiveTabChange(value as RequestDetailTab)}>
  <TabsTrigger value='overview'>...</TabsTrigger>
  <TabsTrigger value='request'>...</TabsTrigger>
  <TabsTrigger value='response'>...</TabsTrigger>
  <TabsTrigger value='executions'>...</TabsTrigger>
</Tabs>
```

Move the existing overview cards into `TabsContent value='overview'`. Do not mount request/response content hooks outside their owning panels.

- [ ] **Step 5: Add request and response content queries inside active panels**

Use:

```ts
const requestContent = useRequestContent(request.id, {
  kind: 'request',
  enabled: activeTab === 'request',
  projectId,
});
const responseContent = useRequestContent(request.id, {
  kind: 'response',
  enabled: activeTab === 'response' && !previewRequest,
  projectId,
});
```

Render local loading/error/retry/empty states. Feed existing JSON/copy/download/cURL/response-flow/chunk code from the focused result rather than metadata.

- [ ] **Step 6: Remove duplicate retained dialog payloads**

Delete `_selectedResponseChunks`. The main chunk dialog reads directly from active response chunks. Ensure `onOpenChange(false)` clears `selectedExecutionChunks`, `curlCommand`, and any selected execution content references.

- [ ] **Step 7: Run focused unit tests**

Run: `cd frontend && node --test src/features/requests/data/request-query-key.test.mjs src/features/requests/data/request-detail-query.test.mjs src/features/requests/components/request-content-state.test.mjs src/features/requests/components/preview-chunk-batcher.test.mjs`

Expected: all tests PASS.

- [ ] **Step 8: Commit**

```bash
git add frontend/src/features/requests/components/request-detail-page.tsx frontend/src/features/requests/components/request-detail-global-page.tsx frontend/src/features/requests/components/request-detail-content.tsx frontend/src/locales/en/requests.json frontend/src/locales/zh-CN/requests.json
git commit -m "feat(requests): lazy load main detail content"
```

### Task 5: Lazy Load One Execution Payload

**Files:**
- Create: `frontend/src/features/requests/components/request-execution-content.tsx`
- Modify: `frontend/src/features/requests/components/request-detail-content.tsx`
- Modify: `frontend/src/locales/en/requests.json`
- Modify: `frontend/src/locales/zh-CN/requests.json`

**Interfaces:**
- Consumes: summary-only `useRequestExecutions`, `useRequestExecutionContent`, `nextExpandedExecution`.
- Produces: `RequestExecutionContentPanel({requestId, executionId, projectId, onShowChunks, onShowCurl})`.

- [ ] **Step 1: Add bilingual execution labels**

English:

```json
"requests.detail.execution.showContent": "View content",
"requests.detail.execution.hideContent": "Hide content"
```

Chinese:

```json
"requests.detail.execution.showContent": "查看内容",
"requests.detail.execution.hideContent": "收起内容"
```

- [ ] **Step 2: Make execution summaries conditional on the Executions tab**

Call `useRequestExecutions(..., {projectId, enabled: activeTab === 'executions'})`. The result must contain no payload fields, as proved in Task 2.

- [ ] **Step 3: Add single-expansion state to the summary list**

```ts
const [expandedExecutionId, setExpandedExecutionId] = useState<string | null>(null);
const toggleExecution = (id: string) => {
  setExpandedExecutionId((current) => nextExpandedExecution(current, id));
};
```

Reset it to `null` when the Executions tab deactivates.

- [ ] **Step 4: Implement `RequestExecutionContentPanel`**

The panel calls `useRequestExecutionContent` with `enabled: true` only while mounted. It owns local loading/error/retry/empty rendering and renders the existing execution request headers/body, response body/chunks, copy/download, cURL, and chunk controls. It must not copy chunks into component state except while a chunk dialog is open; closing the dialog clears that state.

- [ ] **Step 5: Mount only the expanded execution panel**

Below each summary card:

```tsx
<Button onClick={() => toggleExecution(execution.id)}>
  {expandedExecutionId === execution.id
    ? t('requests.detail.execution.hideContent')
    : t('requests.detail.execution.showContent')}
</Button>
{expandedExecutionId === execution.id && (
  <RequestExecutionContentPanel ... />
)}
```

Because only one ID can match, only one execution content query remains active.

- [ ] **Step 6: Run all request feature unit tests**

Run: `cd frontend && pnpm test:unit`

Expected: all Node unit tests PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/features/requests/components/request-execution-content.tsx frontend/src/features/requests/components/request-detail-content.tsx frontend/src/locales/en/requests.json frontend/src/locales/zh-CN/requests.json
git commit -m "feat(requests): lazy load execution payloads"
```

### Task 6: Browser and Runtime Memory Verification

**Files:**
- Modify only if verification reveals a defect in the files from Tasks 1-5.

**Interfaces:**
- Consumes: running frontend at `127.0.0.1:5173`, backend at `127.0.0.1:8090`, optional pprof at `127.0.0.1:6060`.
- Produces: observed network and pprof evidence that payloads load/release at intended boundaries.

- [ ] **Step 1: Run the complete frontend unit suite**

Run: `cd frontend && pnpm test:unit`

Expected: PASS.

- [ ] **Step 2: Verify project detail Overview in the browser**

Navigate to `/project/requests/<id>` with an authorized local account. Confirm the Overview tab is selected, the header/overview render, and network request bodies contain the metadata operation but no `requestHeaders`, `requestBody`, `responseBody`, or `responseChunks` selection.

- [ ] **Step 3: Verify each lazy boundary**

- Click Request: one request-content GraphQL operation appears.
- Click Response: one response-content operation or live-preview request appears.
- Click Executions: one summary-only execution operation appears.
- Expand one execution: one node-scoped execution-content operation appears.
- Expand another: the first panel unmounts and only the second remains.
- Navigate away: no content query remains active and live preview closes.

Confirm copy, JSON download, cURL, chunk dialog, and available audio/video controls still work.

- [ ] **Step 4: Verify administrator detail scope**

Navigate to `/admin/requests/<id>`. Confirm split operations succeed without `X-Project-ID` and administrator-only project/user fields remain visible when permitted.

- [ ] **Step 5: Compare heap profiles**

Capture forced-GC heap profiles before detail, after Overview, after payload load, and after navigation away:

```powershell
Invoke-WebRequest http://127.0.0.1:6060/debug/pprof/heap?gc=1 -OutFile $env:TEMP\request-lazy-before.pb.gz
# drive Overview and content boundaries
Invoke-WebRequest http://127.0.0.1:6060/debug/pprof/heap?gc=1 -OutFile $env:TEMP\request-lazy-after.pb.gz
go tool pprof -top -sample_index=inuse_space -base $env:TEMP\request-lazy-before.pb.gz $env:TEMP\request-lazy-after.pb.gz
```

Expected: Overview does not retain request/response payload allocations; after leaving and forced GC, live heap does not retain the inspected payload or execution content.

- [ ] **Step 6: Check formatting and working tree**

Run:

```bash
git diff --check
git status --short
```

Expected: no whitespace errors; only intended changes if a verification fix remains uncommitted.

- [ ] **Step 7: Commit verification fixes if any**

```bash
git add <only-files-fixed-during-verification>
git commit -m "fix(requests): finalize lazy detail loading"
```

If no verification fix was necessary, do not create an empty commit.

### Task 7: Final Project Verification

**Files:**
- No expected changes.

**Interfaces:**
- Consumes: all implementation commits.
- Produces: final verification evidence and clean repository status.

- [ ] **Step 1: Run request feature unit tests again**

Run: `cd frontend && pnpm test:unit`

Expected: PASS.

- [ ] **Step 2: Run repository Go verification required before commit only if implementation introduced backend Go changes**

No backend Go change is planned. If a backend Go file was changed during implementation, run all required commands from `AGENTS.md` before the final commit:

```bash
go build ./...
cd llm && go build ./...
golangci-lint run --timeout 10m --max-same-issues 50 ./...
cd llm && golangci-lint run --timeout 10m --max-same-issues 50 ./...
go test ./...
cd llm && go test ./...
```

Expected: all PASS.

- [ ] **Step 3: Confirm browser evidence and no console errors**

Repeat one project Overview → Request → Response → Executions → one execution expansion flow and confirm no browser console errors.

- [ ] **Step 4: Confirm clean working tree**

Run: `git status --short`

Expected: no output.

- [ ] **Step 5: Record completion**

Report exact commits, unit-test results, browser paths exercised, network-operation boundaries observed, and pprof before/after evidence. Do not claim frontend build/lint was run because project rules prohibit those commands unless explicitly requested.
