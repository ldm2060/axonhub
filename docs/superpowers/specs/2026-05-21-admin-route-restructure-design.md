# Admin Route Restructure & My Models Page

**Date**: 2026-05-21
**Status**: Approved

## Overview

Restructure frontend routes to clearly separate admin and personal areas, add an independent My Models page, split the dashboard into admin/personal views, and enforce admin page authentication at the route level.

## Background

Current issues:
- Admin pages (channels, models, system, etc.) have no URL-level auth — non-admins can access by typing URLs directly
- "My Models" sidebar entry points to `/my-channels` with no independent page
- Publish requests are in the personal section but only admins can view them
- Dashboard is admin-only; personal users have no landing page
- Admin and personal routes are mixed together with no clear URL structure

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Route organization | TanStack Router layout route (`_admin/`) | Centralized auth guard, automatic protection for new admin pages |
| Admin auth behavior | Redirect to `/` + toast notification | User-friendly, clear feedback |
| My Models data | `ownerID` filter on models query | Same pattern as My Channels |
| Dashboard split | Reuse components + user-level data filtering | Minimal code duplication |
| Admin URL prefix | All admin pages get `/admin` prefix | URL structure matches permission model |

## Route Structure

```
_authenticated/
├── route.tsx                           (AuthGuard + Layout)
├── index.tsx                           → Personal Dashboard (/)
├── _admin/
│   ├── route.tsx                       → AdminGuard + Outlet
│   ├── dashboard.tsx                   → Admin Dashboard (/admin)
│   ├── channels/
│   │   └── index.tsx                   → Admin Channels (/admin/channels)
│   ├── models/
│   │   └── index.tsx                   → Admin Models (/admin/models)
│   ├── publish-requests/
│   │   └── index.tsx                   → Publish Requests (/admin/publish-requests)
│   ├── prompt-protection-rules/
│   │   └── index.tsx                   → (/admin/prompt-protection-rules)
│   ├── data-storages/
│   │   └── index.tsx                   → (/admin/data-storages)
│   └── system/
│       └── index.tsx                   → (/admin/system)
├── my-channels/
│   └── index.tsx                       → My Channels (/my-channels)
├── my-models/
│   └── index.tsx                       → My Models (/my-models) [NEW]
├── shared/
│   └── index.tsx                       → Shared with Me (/shared)
├── project/...                         → Project features (unchanged)
├── settings/...                        → Settings (unchanged)
```

## Admin Authentication (AdminGuard)

**File**: `_admin/route.tsx`

Behavior:
1. Check `authStore.user.isOwner === true`
2. If not owner: redirect to `/` and show toast "No permission to access admin pages"
3. If owner: render `<Outlet />`

This protects all `/admin/*` routes at the URL level. Combined with sidebar visibility filtering (existing `isOwner` check), it provides defense in depth:
- Sidebar: prevents accidental navigation
- Route guard: prevents direct URL access

## My Models Page

**Route**: `/my-models`
**File**: `_authenticated/my-models/index.tsx`
**Permission**: `scopeLevel: 'any'`, no required scopes

Implementation:
- Reuse model components from `features/models`
- Add `ownerID` filter to GraphQL query (same pattern as My Channels)
- Hide admin-only columns (ordering weight, global actions)
- Show personal action buttons (PersonalModelsButtons component)

Sidebar update: change "My Models" entry URL from `/my-channels` to `/my-models`.

## Dashboard Split

### Admin Dashboard (`/admin`)

- Current dashboard content unchanged — system-wide data
- Shows: channel success rates (all), total requests, total users, global metrics
- File moved from `_authenticated/index.tsx` to `_admin/dashboard.tsx`

### Personal Dashboard (`/`)

- Reuses dashboard components with user-level data filtering
- Shows: my channel success rates, my model stats, my request counts
- GraphQL queries filtered by `ownerID`
- If backend APIs don't support user filtering, add filter parameters

## Publish Requests Migration

- Move from `/publish-requests` to `/admin/publish-requests`
- Move from personal sidebar section to admin sidebar section
- Protected by AdminGuard (isOwner check) instead of scope-based guard
- Route file moves from `_authenticated/publish-requests/` to `_authenticated/_admin/publish-requests/`

## Sidebar Structure

### Admin Group (visible only to `isOwner`)

| Label | Route |
|-------|-------|
| Dashboard | `/admin` |
| Channels | `/admin/channels` |
| Models | `/admin/models` |
| Publish Requests | `/admin/publish-requests` |
| Prompt Protection Rules | `/admin/prompt-protection-rules` |
| Data Storages | `/admin/data-storages` |

### Personal Group

| Label | Route |
|-------|-------|
| Dashboard | `/` |
| My Channels | `/my-channels` |
| My Models | `/my-models` |
| Shared with Me | `/shared` |
| API Keys | `/project/api-keys` |
| Prompts | `/project/prompts` |
| Requests | `/project/requests` |
| Traces | `/project/traces` |
| Threads | `/project/threads` |
| Playground | `/project/playground` |

### Settings Group

Unchanged.

## Permission Config Updates

`route-permission.ts` changes:
- All admin routes move to admin group with `scopeLevel: 'system'`
- Add `/my-models` to personal group with `scopeLevel: 'any'`, no required scopes
- Remove `/publish-requests` from personal group, add to admin group
- Add `/admin/dashboard` to admin group

## i18n Updates

Add keys for:
- Admin permission denied toast message
- Any new page titles/headers for My Models page

## Files Changed

### New files
- `frontend/src/routes/_authenticated/_admin/route.tsx` (AdminGuard layout)
- `frontend/src/routes/_authenticated/_admin/dashboard.tsx`
- `frontend/src/routes/_authenticated/_admin/channels/index.tsx`
- `frontend/src/routes/_authenticated/_admin/models/index.tsx`
- `frontend/src/routes/_authenticated/_admin/publish-requests/index.tsx`
- `frontend/src/routes/_authenticated/_admin/prompt-protection-rules/index.tsx`
- `frontend/src/routes/_authenticated/_admin/data-storages/index.tsx`
- `frontend/src/routes/_authenticated/_admin/system/index.tsx`
- `frontend/src/routes/_authenticated/my-models/index.tsx`

### Modified files
- `frontend/src/sidebar.ts` (update URLs, move publish requests, add personal dashboard)
- `frontend/src/config/route-permission.ts` (update route groups, add my-models)
- `frontend/src/routes/_authenticated/index.tsx` (personal dashboard)
- `frontend/src/locales/en.json` (new i18n keys)
- `frontend/src/locales/zh.json` (new i18n keys)

### Deleted files
- `frontend/src/routes/_authenticated/channels/index.tsx` (moved to _admin/)
- `frontend/src/routes/_authenticated/models/index.tsx` (moved to _admin/)
- `frontend/src/routes/_authenticated/publish-requests/index.tsx` (moved to _admin/)
- `frontend/src/routes/_authenticated/prompt-protection-rules/index.tsx` (moved to _admin/)
- `frontend/src/routes/_authenticated/data-storages/index.tsx` (moved to _admin/)
- `frontend/src/routes/_authenticated/system/index.tsx` (moved to _admin/)

Note: Existing route files become the content of new `_admin/` route files. The channel success rates sub-route also moves to `_admin/dashboard/channel-success-rates.tsx`.

## Open Questions

None — all design decisions confirmed.
