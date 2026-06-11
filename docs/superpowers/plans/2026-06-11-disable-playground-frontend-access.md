# Disable Playground Frontend Access Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Disable frontend access to the Playground/Test page while leaving the playground feature implementation and backend/internal code untouched.

**Architecture:** Remove the `/project/playground` frontend route entry point and navigation links, then redirect non-owner login flows to an existing project page. The playground feature component and `/admin/playground/chat` backend/internal logic remain unchanged.

**Tech Stack:** React 19, TanStack Router file-based routes, TypeScript, frontend i18n/sidebar configuration.

---

## File Structure

- Modify/delete: `frontend/src/routes/_authenticated/project/playground/index.tsx` — remove the file-based route entry point so the page is not registered.
- Modify: `frontend/src/routeTree.gen.ts` — regenerate or update generated route tree to remove the playground route import and registration.
- Modify: `frontend/src/sidebar.ts` — remove the Playground/Test page sidebar item pointing to `/project/playground`.
- Modify: `frontend/src/config/route-permission.ts` — remove `/project/playground` from the frontend route permission list.
- Modify: `frontend/src/features/auth/data/auth.ts` — change non-owner login redirect from `/project/playground` to `/project/requests`.
- Do not modify: `frontend/src/features/playground/index.tsx`.
- Do not modify: backend `/admin/playground/chat` implementation or other internal playground code.

## Task 1: Disable Playground Route and Navigation

**Files:**
- Delete or neutralize: `frontend/src/routes/_authenticated/project/playground/index.tsx`
- Modify: `frontend/src/routeTree.gen.ts`
- Modify: `frontend/src/sidebar.ts`
- Modify: `frontend/src/config/route-permission.ts`
- Modify: `frontend/src/features/auth/data/auth.ts`

- [ ] **Step 1: Confirm current references**

Run:

```powershell
rg -n "project/playground|sidebar\.items\.playground|features/playground|admin/playground/chat" frontend/src
```

Expected: references include the route file, generated route tree, sidebar, route permission, auth redirect, and playground feature component/backend endpoint references.

- [ ] **Step 2: Remove the frontend route entry point**

Delete `frontend/src/routes/_authenticated/project/playground/index.tsx` or otherwise ensure it is no longer registered by TanStack Router.

Expected: the route source file for `/_authenticated/project/playground/` no longer contributes a route.

- [ ] **Step 3: Regenerate/update the route tree**

Run the project’s frontend generation/build command that refreshes `frontend/src/routeTree.gen.ts`, or manually remove the generated playground route import, route object, route type entries, and route tree children if generation is unavailable.

Expected: `frontend/src/routeTree.gen.ts` no longer references `AuthenticatedProjectPlaygroundIndexRouteImport`, `/project/playground`, or `/_authenticated/project/playground/`.

- [ ] **Step 4: Remove sidebar access**

In `frontend/src/sidebar.ts`, remove the nav item:

```ts
{
  title: t('sidebar.items.playground'),
  url: '/project/playground',
  icon: IconRobot,
} as NavLink,
```

Expected: the sidebar no longer renders the Playground/Test page link.

- [ ] **Step 5: Remove route permission entry**

In `frontend/src/config/route-permission.ts`, remove the entry:

```ts
{
  path: '/project/playground',
  // Playground is accessible to all users
},
```

Expected: permission config no longer treats `/project/playground` as a valid accessible route.

- [ ] **Step 6: Update non-owner login redirects**

In `frontend/src/features/auth/data/auth.ts`, replace both non-owner redirects:

```ts
const redirectPath = data.user.isOwner ? '/' : '/project/playground';
```

with:

```ts
const redirectPath = data.user.isOwner ? '/' : '/project/requests';
```

Expected: non-owner login and sign-up/session flows land on `/project/requests` instead of the disabled page.

- [ ] **Step 7: Verify internal code remains untouched**

Run:

```powershell
git diff -- frontend/src/features/playground internal | cat
```

Expected: no diff for `frontend/src/features/playground/index.tsx` or backend/internal playground handlers.

- [ ] **Step 8: Run frontend verification**

Run from repo root:

```powershell
cd frontend && pnpm lint
cd frontend && pnpm build
```

Expected: lint/build pass. If script names differ, inspect `frontend/package.json` and run the equivalent available checks.

- [ ] **Step 9: Browser verification**

With the dev server already running, verify:

- Sidebar does not show the Test/Playground item.
- Direct navigation to `http://localhost:5173/project/playground` does not load the playground UI.
- Login redirect logic no longer points non-owner users to `/project/playground` in the built source.

- [ ] **Step 10: Commit**

Run:

```powershell
git add frontend/src/routes/_authenticated/project/playground/index.tsx frontend/src/routeTree.gen.ts frontend/src/sidebar.ts frontend/src/config/route-permission.ts frontend/src/features/auth/data/auth.ts docs/superpowers/plans/2026-06-11-disable-playground-frontend-access.md
git commit -m @'
fix(frontend): disable playground route access

Co-Authored-By: Claude Opus 4.8 (1M context) <noreply@anthropic.com>
'@
```

Expected: commit succeeds without staging `.exe` or backend/internal playground files.

## Self-Review

- Spec coverage: route access, sidebar access, route permission, and login redirects are covered.
- Placeholder scan: no TODO/TBD placeholders remain.
- Type consistency: all paths match discovered files and TanStack route paths.
