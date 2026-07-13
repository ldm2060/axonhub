# Request Detail Lazy Loading Design

**Date:** 2026-07-14

## Goal

Reduce AxonHub server and browser memory pressure when users inspect request records. Opening a request detail route must load metadata only. Large request, response, stream-chunk, and execution payloads load only after an explicit user action and leave the browser query cache promptly when no longer used.

## Scope

This change covers the project and administrator request-detail routes, the shared request-detail component, live response preview, and the GraphQL operations used by those views.

It does not change request persistence, storage layout, retention policy, list-page fields, binary content downloads, or the server-side live-stream registry.

## Current Problem

The current detail operation selects `requestHeaders`, `requestBody`, `responseBody`, and `responseChunks` immediately. The detail component also requests up to ten executions with their complete request and response payloads. gqlgen correctly resolves only selected fields, but these broad selections force the server to read, decode, and serialize all selected content before the user chooses which content to inspect.

React Query then retains the normal detail result under one key, coupling metadata and large payload lifetimes. Live preview also starts at route level and accumulates chunks even when the response tab is not being viewed.

## Chosen Approach

Split detail data into independent GraphQL operations and independent React Query cache entries:

1. Request metadata.
2. Main request content.
3. Main response content.
4. Execution summaries.
5. One execution's content.

This uses gqlgen's existing field-selection behavior to avoid backend changes: fields omitted from an operation are not loaded by their resolvers. It provides meaningful memory isolation without introducing a new REST pagination or streaming protocol.

A single query with conditional `@include` directives was rejected because metadata and content would still share one cached object and lifecycle. New REST content endpoints were deferred because they would require a larger protocol and UI rewrite; they remain an option if individual persisted payloads later prove too large even for on-demand loading.

## User Experience

The detail view has four top-level tabs:

- **Overview** — selected by default; renders metadata already visible in the overview cards and performs no content query.
- **Request** — loads request headers and request body when first selected.
- **Response** — loads response body and response chunks when first selected, or starts live preview for an eligible processing stream.
- **Executions** — loads execution summaries when first selected.

Each execution initially displays summary information only. A user explicitly expands an execution to load its request headers/body and response body/chunks. Collapsing it releases that execution-content query.

Each loading failure remains local to its tab or execution and exposes a retry action; metadata and other tabs remain usable.

## Frontend Data Boundaries

### Request metadata

The metadata operation returns the fields needed by the route header and overview card, including identity, timestamps, project/channel/API-key associations permitted to the viewer, source, model, stream/status/format, storage flags, client IP, latency metrics, and the lightweight usage summary already consumed by the view.

It never selects request headers, request body, response body, or response chunks. Metadata polling for a processing request remains lightweight.

### Main request content

A dedicated query returns only `id`, `requestHeaders`, and `requestBody`. It is enabled only while the Request tab is active. Its key is separate from metadata and response content, and it uses `gcTime: 0`.

### Main response content

A dedicated query returns only `id`, `responseBody`, and `responseChunks`. It is enabled only while the Response tab is active for a persisted/completed response. Its key is separate and uses `gcTime: 0`.

For a processing stream with live preview enabled, the Response tab uses the existing preview endpoint instead. Leaving that tab aborts the preview request, disposes its batcher, and clears the accumulated chunk ref. Completion invalidates/refetches only the relevant metadata and response-content queries.

### Execution summaries

The execution connection query is enabled only while the Executions tab is active. It returns the fields required by the execution cards: identity, timestamps, channel summary, model/format, status/error/status-code fields, request URL when needed for summary actions, pass-through state, and latency metrics. It does not return request headers/body, response body, or response chunks.

### Execution content

A node query keyed by execution ID returns one execution's request headers/body, response body/chunks, request URL, format, and channel fields needed by cURL generation. It is enabled only while that execution is expanded and uses `gcTime: 0`.

Only one execution is expanded at a time. Expanding another collapses the previous one and removes the previous execution-content query. This bounds browser content retention without removing access to any existing functionality.

## Component Structure

The shared detail page owns the active top-level tab. It obtains metadata once and passes it into the shared content component so route headers and content do not register duplicate metadata observers.

The shared content component delegates payload rendering to small tab/content units:

- request-content panel;
- response-content panel;
- execution-summary list;
- execution-content panel.

These units expose explicit loading, error, retry, and empty states. Existing copy, download, cURL, JSON viewer, response-flow, audio/video, and chunk-dialog behavior remains available after the corresponding content has loaded.

Dialog state must not retain a second payload copy. Closing a chunk dialog clears selected execution chunks; the main response dialog reads directly from the active response-content result.

## Cache Lifecycle

- Metadata uses the normal application cache policy so route header and overview navigation remain responsive.
- Main request content, main response content, and individual execution content use `gcTime: 0`.
- Tab deactivation removes inactive content queries explicitly when needed to guarantee prompt release rather than waiting for component teardown.
- Execution summaries may use the normal cache policy because they contain no large payload fields.
- Query keys include permission shape, project scope, administrator-field scope, and content kind to prevent data crossing scopes or colliding with quick-view keys.

## Authorization and Scoping

Project routes continue to pass `X-Project-ID`. Administrator routes continue to use system scope and do not inherit a selected project. Every split operation preserves the existing permission-dependent field selection for project, channel, API-key, and user associations.

No content is moved to persistent browser storage. React Query holds it only in memory while the relevant view is active.

## Error and Cancellation Behavior

- Each content query reports a local error and retry action.
- Navigating away or deactivating a content view cancels its active fetch through TanStack Query where possible and removes the content cache entry.
- Live preview always aborts its `fetch`, clears reconnect timers, disposes batching, and empties the chunk ref when the Response tab deactivates or the route unmounts.
- A completed live request refreshes metadata and allows persisted response content to load without merging large payloads into metadata.

## Verification

Automated coverage will verify:

1. Metadata operations do not contain large payload fields.
2. Request and response content operations select only their intended payload fields.
3. Execution summary operations omit payload fields.
4. One execution-content operation targets one execution ID.
5. Query keys isolate metadata, content kind, execution ID, project scope, and permission shape.
6. Content queries are disabled before their tab or execution is activated and use immediate garbage collection.
7. Live preview activation and cleanup follow Response-tab visibility.

Browser verification will exercise project and administrator detail routes, all four tabs, one execution expansion, copy/download/cURL/chunk actions, navigation away, and a processing-stream preview when available. Network inspection must show no main payload on Overview and no execution payload until expansion.

Runtime verification will compare pprof heap profiles and process memory before opening a detail route, after Overview, after loading content, and after leaving the route plus forced GC. The acceptance criterion is that Overview does not allocate retained request/response payloads, and post-GC live heap does not retain content from closed tabs or departed detail routes.

## Success Criteria

- Opening a request detail route performs no query that selects request/response payloads or execution payloads.
- A payload is fetched only after the user activates its owning tab or expands its execution.
- At most one execution payload is retained by the browser at a time.
- Leaving a payload view promptly releases its browser query cache and live-preview buffers.
- Existing project/admin authorization, live preview, binary download, JSON inspection, copying, downloading, and cURL generation continue to work.
- Browser and pprof verification show the intended allocation and release behavior.
