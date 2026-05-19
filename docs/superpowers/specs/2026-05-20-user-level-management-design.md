# User-Level Management Design

**Date:** 2026-05-20
**Status:** Draft
**Approach:** Lightweight retrofit (Approach A)

## Goal

Transform AxonHub from project-based isolation to user-level management. Each non-admin user sees only their own resources (channels, models, API keys, prompts, requests). Users can manage their own channels and models without admin permissions, share with specific users, or request public publishing via admin review.

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Project removal | Keep Project table, auto-create per user | Reuse existing isolation logic (Privacy Policy, hooks, resolvers) |
| Channel/Model ownership | Add `owner_id` + `visibility` + `shared_with` fields | Minimal schema change, clear ownership semantics |
| Sharing model | Dual: p2p sharing + admin-reviewed publishing | Meets both collaborative and governance needs |
| Registration | Configurable via system settings | Flexibility for different deployment scenarios |
| Dashboard | Dual pages: per-user + global | Admin gets oversight, users get personal visibility |

## Data Model Changes

### Channel Schema (`internal/ent/schema/channel.go`)

New fields:

| Field | Type | Description |
|-------|------|-------------|
| `owner_id` | int, optional | Creator user ID. `nil` = system/admin-created global channel |
| `visibility` | enum: `private`, `shared`, `published` | `private`=owner only, `shared`=owner + shared users, `published`=all users |
| `shared_with` | JSON int[] | User IDs this resource is shared with |

### Model Schema (`internal/ent/schema/model.go`)

Same three fields as Channel: `owner_id`, `visibility`, `shared_with`.

### User Schema (`internal/ent/schema/user.go`)

New field:

| Field | Type | Description |
|-------|------|-------------|
| `private_project_id` | int, optional | Auto-created private project ID (cached to avoid repeated lookups) |

### PublishRequest Schema (`internal/ent/schema/publish_request.go`) — New Entity

| Field | Type | Description |
|-------|------|-------------|
| `resource_type` | enum: `channel`, `model` | Resource type being published |
| `resource_id` | int | Resource ID |
| `requester_id` | int | Requesting user |
| `status` | enum: `pending`, `approved`, `rejected` | Review status |
| `reviewer_id` | int, optional | Reviewing admin |
| `review_comment` | string, optional | Review notes |
| `created_at` / `updated_at` | time | Timestamps |

Edges: `requester` (User), `reviewer` (User, optional).

## Project Automation

- On user registration (or first login for existing users), auto-create a private project with name `__user_{user_id}__`.
- Store the project ID in `user.private_project_id`.
- Frontend never exposes project UI (no switcher, no list, no guard).
- All existing `project_id`-based isolation logic continues to work unchanged.
- Backend automatically resolves `project_id` from the authenticated user's `private_project_id` instead of requiring `X-Project-ID` header.

## User Registration

### GraphQL API

```graphql
input SignUpInput {
  email: String!
  password: String!
  firstName: String
  lastName: String
}

type SignUpPayload {
  user: User!
  token: String!
}

extend type Mutation {
  signUp(input: SignUpInput!): SignUpPayload!
}
```

### System Settings

| Setting Key | Type | Default | Description |
|-------------|------|---------|-------------|
| `allow_sign_up` | bool | `true` | Whether self-registration is allowed |
| `sign_up_approval_required` | bool | `false` | Whether admin approval is required after registration |

### Registration Modes

| Mode | Behavior |
|------|----------|
| `allow_sign_up=false` | Registration endpoint returns error. Users created only via admin `createUser` or OIDC linking. |
| `allow_sign_up=true` + `sign_up_approval_required=false` | Register → create user + private project → status `activated` → return JWT immediately. |
| `allow_sign_up=true` + `sign_up_approval_required=true` | Register → create user + private project → status `deactivated` → admin approves in user management page. |

### OIDC JIT Provisioning

When `allow_sign_up=false`, OIDC auto-creation is also blocked. Only existing user linking is allowed.

### Frontend

- New route `/sign-up` with email, password, name form.
- When `allow_sign_up=false`, show "Registration is closed" message.
- Admin user management page adds `status=deactivated` filter for approval queue.

## Visibility Model

### Channel/Model Visibility States

```
private → (share) → shared → (publish approved) → published
                                              ↗
private ────────────(publish approved)─────────↗
published → (unpublish) → private
shared → (remove all shares) → private
```

- `private`: Only owner can see and use.
- `shared`: Owner + users in `shared_with` can see and use (read-only for non-owners).
- `published`: All users can see and use (read-only for non-owners).

### Peer-to-Peer Sharing

- Owner searches and selects users in resource detail page.
- Updates `shared_with` JSON field and sets `visibility = "shared"`.
- Shared users see the resource with a "Shared with me" tag.
- Shared users can use the resource (route requests through it) but cannot edit, re-share, or manage it.
- Removing all shared users reverts visibility to `private`.
- `shared_with` uses a JSON int array. Adequate for typical use cases (<50 shared users per resource). If scaling becomes an issue, a separate join table can replace it later without API changes.

### Publishing Review

1. User clicks "Request public publishing" on their resource.
2. Optionally provides a comment/description.
3. System creates `PublishRequest` with `status: pending`.
4. Admin reviews in dedicated review queue page.
5. **Approve**: Resource `visibility` → `published`, all users can see it.
6. **Reject**: Resource keeps current `visibility`, admin can leave a review comment.
7. User can cancel their own pending request.
8. Admin or owner can unpublish a `published` resource, reverting it to `private` (clearing `shared_with`).
9. A resource in `shared` state can be submitted for publishing directly — no need to unshare first. The `shared_with` list is preserved during review; on approval, `shared_with` is cleared since `published` supersedes sharing.

## Permission System Changes

### New Scopes

| Scope | Level | Description |
|-------|-------|-------------|
| `manage_own_channels` | system | Manage channels where `owner_id = current user` |
| `manage_own_models` | system | Manage models where `owner_id = current user` |
| `review_publish_requests` | system | Review publish requests (admin) |

### Default Scope Assignment

| Role | Scopes |
|------|--------|
| New registered user | `manage_own_channels`, `manage_own_models`, `read_channels`, `read_models`, `read_api_keys`, `write_api_keys`, `read_requests`, `write_requests`, `read_prompts`, `write_prompts` |
| Admin (existing) | All scopes including `write_channels`, `write_models`, `review_publish_requests` |
| System owner (`is_owner`) | Bypass all checks |

### Channel Privacy Policy

**Read rules** (evaluated in order):
1. System owner → Allow
2. `write_channels` scope → Allow (admin sees all)
3. `visibility = "published"` → Allow
4. `owner_id = current user` → Allow
5. `shared_with` contains current user ID → Allow
6. Default: Deny

**Write rules**:
1. System owner → Allow
2. `write_channels` scope → Allow (admin edits all)
3. `manage_own_channels` scope + `owner_id = current user` → Allow
4. Default: Deny

Model privacy policy follows the same pattern.

### Project-Level Scopes

Project-level scopes (`read_api_keys`, `write_api_keys`, `read_requests`, `write_requests`, `read_prompts`, `write_prompts`, `read_users`, `write_users`, `read_roles`, `write_roles`) remain unchanged. They are automatically resolved using the user's private project instead of requiring project selection.

## Frontend Changes

### Navigation Restructure

**Current → New (normal user):**

| Current | New |
|---------|-----|
| Dashboard (global) | My Dashboard (user-scoped) |
| Projects | *(removed)* |
| Channels (admin only) | My Channels + Discover (public) |
| Models (admin only) | My Models + Discover (public) |
| /project/api-keys (requires project select) | API Keys (auto-scoped to user) |
| /project/prompts | Prompts |
| /project/requests | Requests |
| /project/traces | Traces |
| /project/threads | Threads |
| /project/playground | Playground |

**Admin additional navigation:**
- Global Dashboard (all data)
- User Management
- Review Queue (publish requests)
- All Channels / All Models management
- System Settings

### Components Removed

- `ProjectSwitcher` — no project selection needed
- `ProjectGuard` — no project guard needed
- `projectStore` selection state — replaced with `useCurrentUser().privateProjectId`

### Components Modified

- All data hooks using `useSelectedProjectId()` → replace with `useCurrentUser().privateProjectId`
- `X-Project-ID` header → auto-injected from user context
- `where.projectID` filters → auto-populated from user context
- Channel list page → shows own + shared + published channels
- Model list page → shows own + shared + published models
- Channel/Model detail pages → add sharing UI and publish request button
- Dashboard → two pages: `/dashboard` (my data) and `/admin/dashboard` (global, admin only)

### New Pages

- `/sign-up` — Registration page
- `/review-queue` — Admin publish request review (admin only)
- `/admin/dashboard` — Global dashboard (admin only)

### i18n

New keys needed for:
- Visibility labels (private, shared, published)
- Sharing UI (search users, shared with me)
- Publish request flow (request, cancel, approve, reject)
- Registration page
- My Dashboard vs Global Dashboard
- Review Queue

## Dashboard Design

### My Dashboard (`/dashboard`) — All Users

Data source: filtered by user's `private_project_id`.

Metrics:
- Total requests (user's)
- Failed requests (user's)
- Average response time
- Token consumption (prompt, completion, total)
- Cost stats
- Request trends (7 days)
- Distribution by model, by channel
- Top API keys by usage

### Global Dashboard (`/admin/dashboard`) — Admin Only

Data source: no project filter (current behavior unchanged).

All existing global metrics preserved. Additional breakdown by user added.

## GraphQL API Additions

```graphql
# Sharing
mutation shareChannel(id: ID!, userIDs: [ID!]!): Channel!
mutation unshareChannel(id: ID!, userIDs: [ID!]!): Channel!
mutation shareModel(id: ID!, userIDs: [ID!]!): Model!
mutation unshareModel(id: ID!, userIDs: [ID!]!): Model!

# Publishing
mutation requestPublish(resourceType: ResourceType!, resourceID: ID!, comment: String): PublishRequest!
mutation cancelPublishRequest(id: ID!): Boolean!
mutation reviewPublishRequest(id: ID!, action: ReviewAction!, comment: String): PublishRequest!

# Queries
query publishRequests(status: PublishRequestStatus, resourceType: ResourceType): [PublishRequest!]!
query mySharedChannels: [Channel!]!
query mySharedModels: [Model!]!

# Registration
mutation signUp(input: SignUpInput!): SignUpPayload!

# Enums
enum ResourceType { channel, model }
enum ReviewAction { approve, reject }
enum PublishRequestStatus { pending, approved, rejected }
enum Visibility { private, shared, published }
```

## Data Migration

### Strategy

Use Ent's DataMigrate (`internal/ent/migrate/datamigrate/`).

| Step | Operation |
|------|-----------|
| 1 | Set all existing Channels: `owner_id = NULL, visibility = "published"` |
| 2 | Set all existing Models: `owner_id = NULL, visibility = "published"` |
| 3 | For each User without a private project: create one (`__user_{user_id}__`) and set `private_project_id` |
| 4 | Migrate default project (project_id=1) resources to per-user private projects based on API Key `user_id` |

### Backward Compatibility

- Existing API keys remain valid — `project_id` points to user's private project.
- Existing request logs unchanged — still associated via `project_id`.
- LLM request processing pipeline unchanged — project_id isolation logic preserved.
- Existing admin workflows unchanged — `write_channels`/`write_models` still grant full access.

### User Deletion

When a user is deleted (soft delete):
- Their private project is soft-deleted.
- Their channels/models with `visibility = "private"` are soft-deleted.
- Their channels/models with `visibility = "shared"` have the deleted user removed from other users' `shared_with` lists. If no shared users remain, visibility reverts to `private`.
- Their channels/models with `visibility = "published"` are reassigned `owner_id = NULL` (become system-owned) to avoid disrupting other users. Admin is notified.

## Files to Create/Modify

### Backend — New Files

- `internal/ent/schema/publish_request.go`
- `internal/server/biz/publish_request.go`
- `internal/server/gql/publish_request.graphql`
- `internal/server/gql/publish_request.resolvers.go`
- `internal/server/biz/signup.go`

### Backend — Modified Files

- `internal/ent/schema/channel.go` — add `owner_id`, `visibility`, `shared_with`
- `internal/ent/schema/model.go` — add `owner_id`, `visibility`, `shared_with`
- `internal/ent/schema/user.go` — add `private_project_id`
- `internal/scopes/scopes.go` — add new scopes
- `internal/scopes/rule_*.go` — update Channel/Model read/write rules
- `internal/server/biz/user.go` — auto-create private project on registration
- `internal/server/biz/oidc.go` — adapt OIDC JIT provisioning
- `internal/server/biz/channel.go` — visibility-aware queries
- `internal/server/biz/model.go` — visibility-aware queries
- `internal/server/gql/axonhub.graphql` — new mutations, queries, types
- `internal/server/gql/dashboard.resolvers.go` — user-scoped dashboard
- `internal/ent/migrate/datamigrate/migrator.go` — data migration

### Frontend — New Files

- `frontend/src/routes/_authenticated/sign-up/` — registration page
- `frontend/src/features/publish-requests/` — admin review queue
- `frontend/src/features/my-dashboard/` — user dashboard (or refactor existing)

### Frontend — Modified Files

- `frontend/src/sidebar.ts` — navigation restructure
- `frontend/src/stores/projectStore.ts` — simplify to auto-resolve
- `frontend/src/components/layout/project-switcher.tsx` — remove
- `frontend/src/components/project-guard.tsx` — remove
- `frontend/src/features/channels/` — add sharing/publishing UI
- `frontend/src/features/models/` — add sharing/publishing UI
- `frontend/src/features/dashboard/` — dual dashboards
- `frontend/src/locales/en.json` + `zh.json` — new translations
