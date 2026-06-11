# Admin All Requests Page Design

## Context

AxonHub already has a project-scoped request log page at `/project/requests` and request detail pages that show request metadata, request and response bodies, executions, usage, latency, cost, and retry information. The current page intentionally scopes data to the selected project by passing project context and injecting `projectID` into the GraphQL request filter.

Administrators need a page in the admin section that can view request records across all users and projects. The page should reuse the existing request log experience while making the data scope explicitly global.

## Goals

- Add an admin navigation entry and route for viewing all request records.
- Restrict the new admin page to system owners only.
- Show all projects' requests without depending on the currently selected project.
- Show ownership context in the table, including user and API key, and include project for traceability.
- Preserve existing request table behavior: cursor pagination, filters, auto-refresh, drawer preview, and full detail view.
- Allow owners to view complete request details, including request and response bodies.

## Non-goals

- Adding request deletion, retry, export, or mutation actions.
- Changing how project-scoped request pages work.
- Introducing a new backend permission model for this page.
- Redesigning request storage or request capture behavior.

## Recommended approach

Reuse the existing request log feature with an explicit global/admin mode.

The admin route should render the same request management feature used by the project request page, but configure it so queries are not scoped to the selected project. This avoids duplicating table, filter, drawer, and detail logic while keeping the admin page behavior clear.

The backend GraphQL `requests` query already supports owner access to requests across projects through the existing Ent policy. The `Request` GraphQL type also exposes related `project`, `apiKey`, and `apiKey.user` data, so a dedicated `adminRequests` API is not necessary for this feature.

## Routes and navigation

Add:

- `/admin/requests` for the global request list.
- Prefer `/admin/requests/$requestId` for admin request details so the back button returns to the admin request list and the URL matches the admin context.

Update the admin sidebar:

- Add a `Requests` entry under the Admin group.
- Use the existing `sidebar.items.requests` i18n key.
- The entry should only appear for owners because the whole admin route tree is owner-only.

Update route permission config if needed so route filtering does not hide `/admin/requests` for owners. Since `/admin` currently uses an owner guard, the simplest rule is to keep this route aligned with the existing admin owner-only behavior instead of broadening it to non-owner users with system `read_requests`.

## Component design

Extend the existing request feature rather than creating a separate feature tree.

Recommended component shape:

- `RequestsManagement` accepts an optional mode/scope prop, for example `scope="project" | "admin"`.
- Project mode keeps today's behavior:
  - uses the selected project ID,
  - passes `X-Project-ID`,
  - injects `projectID` into the request list filter,
  - navigates to `/project/requests/$requestId`.
- Admin mode changes only the necessary scope behavior:
  - does not pass `X-Project-ID`,
  - does not inject `projectID`,
  - loads global filter option lists where needed,
  - navigates to `/admin/requests/$requestId`,
  - uses global detail queries.

The request table should receive enough context to render extra admin columns without affecting the project page. A simple option such as `showOwnershipColumns` can enable the admin-only columns.

## Table columns

The admin request table should include these ownership columns:

1. **User**
   - Source: `request.apiKey.user`.
   - Display priority: full name if available, then email, then `-`.
   - If the request has no API key or the API key has no user, show `-`.
2. **API Key**
   - Source: `request.apiKey.name`.
   - If unavailable, show `-`.
3. **Project**
   - Source: `request.project.name`.
   - This is included even though the user asked for user + key because it prevents global rows from becoming ambiguous across projects.
   - If unavailable, show `-`.

Existing columns such as model, format, stream, source, client IP, channel, status, tokens, cache, cost, latency, details, and created time should continue to behave as they do today.

## Data flow

### Request list

Admin mode should query `requests` with the same pagination and sorting arguments as the project page:

- order by `CREATED_AT DESC`,
- cursor pagination,
- same table filters where supported by GraphQL.

Admin mode must not add a project filter unless the user explicitly filters by project in a future enhancement. For this initial feature, the page is global by default and the existing filters remain focused on status, source, channel, API key, model ID, and created time.

The request list query should include:

- `project { id name }`,
- `apiKey { id name user { id email firstName lastName } }`,
- existing request list fields.

### Filter options

Channel and API key filter options need to work globally in admin mode:

- Channel options should be loaded without binding to the selected project.
- API key options should be loaded without requiring a selected project.

If the existing API key hook requires a selected project, add an option for admin/global usage rather than creating a parallel hook. The option should preserve the current project-scoped default behavior.

### Detail view

Admin details should use global queries:

- No `X-Project-ID` header.
- No selected-project dependency.
- Full request body and response body remain available to owners.

The drawer's previous/next navigation should reuse the same global `where` filter and no project header, so moving between rows stays within the admin global result set.

## Permissions and security

- Only system owners can access `/admin/requests`.
- The existing `/admin` route guard already redirects non-owners; the new route should inherit that behavior.
- The sidebar entry should not appear for non-owners.
- The page is read-only. It should not add request deletion, retry, or mutation actions.
- Complete request and response bodies are visible to owners because the user explicitly approved full details.
- Missing related data should degrade gracefully instead of hiding the row or failing the page.

## Error handling

- Reuse existing `useErrorHandler` behavior for GraphQL failures.
- Show `-` for missing user, API key, project, or channel data.
- If a request detail node is missing, reuse the existing request-not-found or load-failed experience.
- If global filter option loading fails, the table should still render request rows and surface the existing error toast.

## Testing strategy

Manual/browser verification should cover:

1. Owner sees a `Requests` entry in the admin sidebar.
2. Non-owner access to `/admin/requests` is blocked by the admin guard.
3. `/admin/requests` loads without relying on the selected project.
4. Requests from multiple projects can appear on the same page.
5. User, API Key, and Project columns display expected values and use `-` for missing data.
6. Status, source, channel, API key, model ID, and date filters still work.
7. Cursor pagination and auto-refresh still work.
8. Drawer preview works in admin mode, including previous/next navigation across pages.
9. Full detail page opens from admin mode and can show request body, response body, executions, and usage.
10. The admin detail back action returns to `/admin/requests` with the prior search state when practical.

Automated checks should focus on TypeScript/build-time safety and targeted unit tests only if existing test structure makes that practical. No backend schema generation should be required if the implementation only consumes existing GraphQL fields.