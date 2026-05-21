# Registration, Email & Sidebar Redesign

## Context

AxonHub currently has a basic self-registration system gated by `allow_sign_up` and `sign_up_approval_required` system settings, but it lacks email verification, password reset, and admin notification. The forgot-password page is a non-functional stub. The dashboard has a redundant Project/Personal toggle that duplicates the sidebar navigation. User and role management pages exist but are not accessible from the admin sidebar.

This design consolidates navigation, adds email infrastructure, and builds a complete registration + approval + password-reset flow.

## 1. Sidebar & Dashboard UI

### Remove dashboard toggle

- Remove the `<Tabs>` component (Project/Personal) from `frontend/src/features/dashboard/index.tsx`
- Remove `DashboardModeContext` and `useDashboardMode()` — each dashboard page hardcodes its mode
- Personal dashboard (`/`) always uses `myDashboard` query
- Admin dashboard (`/admin`) always uses `dashboardOverview` query

### Add Users & Roles to admin sidebar

In `frontend/src/sidebar.ts`, add to the admin group (after Data Storages):

| Item | URL | Icon |
|------|-----|------|
| Users | `/admin/users` | `IconUsers` |
| Roles | `/admin/roles` | `IconUsersGroup` |

- Users item shows a **pending-approval count badge** when `pending` users exist (fetched via a lightweight GraphQL query or included in `me` response)
- Move existing `/users` and `/roles` routes to `/admin/users` and `/admin/roles` (they were already admin-only by scope requirement)
- Update `frontend/src/routes/_authenticated/users/` and `frontend/src/routes/_authenticated/roles/` route paths

### Files to modify

- `frontend/src/features/dashboard/index.tsx` — remove tabs, hardcode mode
- `frontend/src/features/dashboard/context.tsx` — remove context provider
- `frontend/src/routes/_authenticated/index.tsx` — remove DashboardModeContext.Provider
- `frontend/src/sidebar.ts` — add Users, Roles items to admin group
- `frontend/src/routes/_authenticated/users/index.tsx` — update path
- `frontend/src/routes/_authenticated/roles/index.tsx` — update path

## 2. Registration Settings

### New "Registration" tab in System Settings

Location: `frontend/src/features/system/components/tabs.tsx` — add `registration` tab.

Settings stored in `system` table as key `registration_settings` (JSON):

```go
type RegistrationSettings struct {
    AllowSignUp           bool   `json:"allow_sign_up"`
    ApprovalRequired     bool   `json:"approval_required"`
    DefaultUserScopes     []string `json:"default_user_scopes"`
}
```

This replaces the existing separate `allow_sign_up` and `sign_up_approval_required` keys. Migration: read old keys, write to new consolidated key, delete old keys.

### UI elements

- Toggle: Allow self-registration
- Mode selector: Auto-approve / Require admin approval (only visible when registration is enabled)
- Default scopes: multi-select from available scopes (reuses existing `DefaultUserScopes` from `signup.go`)

### GraphQL

- New type `RegistrationSettings`
- New query `registrationSettings`
- New mutation `updateRegistrationSettings(input: RegistrationSettingsInput!)`

## 3. Email Service

### New "Email" tab in System Settings

Settings stored in `system` table as key `email_settings` (JSON):

```go
type EmailSettings struct {
    SMTPHost     string `json:"smtp_host"`
    SMTPPort     int    `json:"smtp_port"`
    SMTPUser     string `json:"smtp_user"`
    SMTPPassword string `json:"smtp_password"`
    Encryption   string `json:"encryption"` // "ssl" | "starttls" | "none"
    FromName     string `json:"from_name"`
    FromAddress  string `json:"from_address"`
}
```

### UI elements

- SMTP Host, Port, Username, Password fields
- Encryption: SSL/TLS (port 465) / STARTTLS (port 587) / None
- From Address: name + email (e.g., "AxonHub <noreply@example.com>")
- Connection status indicator (green/red dot)
- "Send Test Email" button — calls `testEmailConnection` mutation which attempts to dial + auth, sends a test message to the current admin's email

### GraphQL

- New type `EmailSettings`
- New query `emailSettings` (password masked)
- New mutation `updateEmailSettings(input: EmailSettingsInput!)`
- New mutation `testEmailConnection` — returns `{success, message}`

### EmailService (`internal/server/biz/email.go`)

- `NewEmailService(systemService, brandName)` — loads config from `system` table, cached via `xcache`
- `Send(ctx, to, subject, htmlBody, textBody)` — core send method
- `SendVerificationEmail(ctx, user, tokenURL)` — renders template, calls Send
- `SendPasswordResetEmail(ctx, user, tokenURL)` — renders template, calls Send
- `SendAdminNotification(ctx, user)` — notifies all owners about new pending user
- `SendApprovedEmail(ctx, user, signInURL)` — notifies user of approval
- `SendRejectedEmail(ctx, user)` — notifies user of rejection
- `SendTestEmail(ctx, to)` — sends a simple test message
- SMTP connection: `net/smtp` with `PLAIN` auth, dial per-send (simple, reliable)
- Templates: Go `html/template` files embedded via `embed.FS`, branded with `brand_name` and `brand_logo`

### Email templates

Four templates, each with HTML + plain text variant, stored in `internal/server/biz/email/templates/`:

1. `verify_email.html` / `verify_email.txt` — "Verify your email" with action button/link
2. `reset_password.html` / `reset_password.txt` — "Reset your password" with action link
3. `account_approved.html` / `account_approved.txt` — "Your account has been approved" with sign-in link
4. `account_rejected.html` / `account_rejected.txt` — "Your registration was not approved"

All templates include `{{.BrandName}}`, `{{.BrandLogoURL}}`, `{{.ActionURL}}`, `{{.RecipientName}}`.

## 4. Email Token System

### New `email_token` Ent schema

File: `internal/ent/schema/email_token.go`

```go
fields:
- token       string  (unique, indexed, random UUID)
- type        enum    ("verify_email", "reset_password")
- expires_at  time
- consumed_at time    (optional, nullable)

edges:
- user        (required, FK → user)
```

Policy: only system-level access (not user-facing GraphQL — tokens are consumed via REST).

### EmailTokenService (`internal/server/biz/email_token.go`)

- `CreateToken(ctx, userID, tokenType) → (token string, error)` — generates UUID, stores with 24h expiry
- `ValidateToken(ctx, token, tokenType) → (userID int, error)` — checks type, expiry, not consumed
- `ConsumeToken(ctx, token) → error` — sets `consumed_at = now`
- `CleanupExpired(ctx)` — called by GC cron, deletes tokens past expiry

## 5. User Status Changes

### Updated status enum

Current: `activated`, `deactivated`
New: `activated`, `deactivated`, `pending`

- `pending` — user has registered but either email is unverified or (if approval required) awaiting admin approval
- New field on user schema: `email_verified_at` (optional timestamp, nullable)
- A `pending` user with `email_verified_at = null` has not verified their email
- A `pending` user with `email_verified_at` set has verified email but awaits admin approval

### Login behavior

- Only `activated` users can sign in
- `pending` users get "Your account is not yet active. Please verify your email or wait for admin approval."
- `deactivated` users get "Your account has been deactivated."

## 6. New REST Endpoints

| Method | Path | Auth | Handler | Description |
|--------|------|------|---------|-------------|
| GET | `/admin/auth/verify-email` | None | `VerifyEmail` | Validates token, marks email verified, redirects to frontend with status param |
| POST | `/admin/auth/resend-verification` | None | `ResendVerification` | Takes `{email}`, rate-limited 1/min per email via in-memory map, resends verification email |
| POST | `/admin/auth/forgot-password` | None | `ForgotPassword` | Creates reset token, sends reset email |
| POST | `/admin/auth/reset-password` | None | `ResetPassword` | Validates token, sets new password, consumes token |

Existing `POST /admin/auth/signup` is updated to create `pending` users and send verification email instead of auto-signing in.

### Redirect behavior for verify-email

On success (auto-approve mode):
- Redirect to `/sign-in?verified=1` — frontend shows "Email verified, please sign in"

On success (approval mode):
- Redirect to `/sign-in?verified=1&pending=1` — frontend shows "Email verified, awaiting admin approval"

On failure (invalid/expired token):
- Redirect to `/sign-in?verified=0` — frontend shows "Verification link invalid or expired"

## 7. New GraphQL Mutations

| Mutation | Input | Description |
|----------|-------|-------------|
| `approveUser` | `{id!}` | Sets status=activated, assigns default scopes, sends approved email |
| `rejectUser` | `{id!}` | Deletes user record, sends rejected email |
| `updateRegistrationSettings` | `RegistrationSettingsInput` | Updates registration config |
| `updateEmailSettings` | `EmailSettingsInput` | Updates SMTP config |
| `testEmailConnection` | `{}` | Tests current SMTP config, sends to current user |

### Updated queries

- `users` query — add `status` filter support (to filter for `pending`)
- `registrationSettings` — returns current registration settings
- `emailSettings` — returns current email settings (password masked)

## 8. Frontend Route Changes

| Route | Component | Status |
|-------|-----------|--------|
| `/sign-up` | Existing, updated | No auto-login after signup, show "check email" |
| `/verify-email` | New | Handles `?verified=1&pending=1` params, shows appropriate message |
| `/forgot-password` | Existing stub, replace | Real implementation with email flow |
| `/reset-password` | New | Token-validated password reset form |
| `/admin/users` | Moved from `/users` | User management with approve/reject |
| `/admin/roles` | Moved from `/roles` | Role management |

### Sign-in page updates

- Show "Email verified! Please sign in." toast when `?verified=1`
- Show "Awaiting admin approval." message when `?verified=1&pending=1`
- "Forgot password?" link now works (sends reset email)

## 9. Registration Flow Summary

### Auto-approve mode

```
User fills /sign-up form
  → POST /admin/auth/signup
  → User created (status=pending, email_verified_at=null)
  → Verification email sent
  → Frontend shows "Check your email"

User clicks verification link
  → GET /admin/auth/verify-email?token=...
  → Token validated + consumed
  → email_verified_at = now, status = activated
  → Redirect to /sign-in?verified=1

User signs in normally
```

### Approval-required mode

```
User fills /sign-up form
  → POST /admin/auth/signup
  → User created (status=pending, email_verified_at=null)
  → Verification email sent
  → Frontend shows "Check your email"

User clicks verification link
  → GET /admin/auth/verify-email?token=...
  → Token validated + consumed
  → email_verified_at = now, status stays pending
  → Admin notification email sent
  → Redirect to /sign-in?verified=1&pending=1

Admin clicks Approve on /admin/users
  → approveUser mutation
  → status = activated, default scopes assigned
  → Approved email sent to user

User signs in normally
```

### Password reset flow

```
User clicks "Forgot password?" on /sign-in
  → /forgot-password form
  → POST /admin/auth/forgot-password
  → Reset token created, reset email sent
  → Frontend shows "Check your email"

User clicks reset link
  → /reset-password?token=...
  → Frontend validates token, shows password form
  → POST /admin/auth/reset-password
  → Password updated, token consumed
  → Redirect to /sign-in with success message
```

## 10. Key Files to Create/Modify

### New files

- `internal/ent/schema/email_token.go` — Ent schema
- `internal/server/biz/email.go` — EmailService
- `internal/server/biz/email_token.go` — EmailTokenService
- `internal/server/biz/email/templates/*.html` + `*.txt` — Email templates
- `internal/server/api/email_token.go` — REST handlers for verify-email, forgot-password, reset-password, resend-verification
- `frontend/src/routes/(auth)/verify-email.tsx` — Email verification result page
- `frontend/src/routes/(auth)/reset-password.tsx` — Password reset form
- `frontend/src/features/system/components/registration-settings.tsx` — Registration settings UI
- `frontend/src/features/system/components/email-settings.tsx` — Email settings UI

### Modified files

- `frontend/src/features/dashboard/index.tsx` — Remove tabs, hardcode mode
- `frontend/src/features/dashboard/context.tsx` — Remove context
- `frontend/src/routes/_authenticated/index.tsx` — Remove DashboardModeContext.Provider
- `frontend/src/sidebar.ts` — Add Users, Roles to admin group
- `frontend/src/routes/_authenticated/users/index.tsx` — Update path
- `frontend/src/routes/_authenticated/roles/index.tsx` — Update path
- `frontend/src/features/auth/sign-up/` — Remove auto-login, show "check email"
- `frontend/src/features/auth/forgot-password/` — Replace stub with real implementation
- `frontend/src/features/auth/sign-in/` — Handle verified/pending params, link forgot-password
- `frontend/src/features/system/components/tabs.tsx` — Add Registration, Email tabs
- `internal/ent/schema/user.go` — Add `email_verified_at` field, `pending` status
- `internal/server/biz/signup.go` — Create pending users, send verification email
- `internal/server/biz/user.go` — Add ApproveUser, RejectUser methods
- `internal/server/biz/system.go` — Add RegistrationSettings, EmailSettings types + CRUD
- `internal/server/routes.go` — Register new REST endpoints
- `internal/server/gql/system.graphql` — Add new types, queries, mutations
- `internal/server/gql/system.resolvers.go` — Add resolvers
- `internal/server/gql/user.graphql` — Add approveUser, rejectUser mutations
- `internal/server/gql/user.resolvers.go` — Add resolvers
- `frontend/src/gql/` — Regenerate GraphQL types

## 11. Verification

1. **Sidebar**: Open app, verify no Project/Personal toggle on dashboard, verify Users/Roles appear in admin sidebar
2. **Registration settings**: Go to /admin/system → Registration tab, toggle settings, verify they persist
3. **Email settings**: Go to /admin/system → Email tab, configure SMTP, click "Send Test Email", verify delivery
4. **Registration (auto-approve)**: Sign up as new user, verify email received, click link, verify redirected to sign-in with success message, verify can sign in
5. **Registration (approval-required)**: Sign up as new user, verify email, click link, verify "awaiting approval" message, verify admin gets notification, admin approves, verify user gets approval email, verify user can sign in
6. **Rejection**: Admin rejects a pending user, verify user gets rejection email, verify user record is deleted
7. **Forgot password**: Click "Forgot password?", enter email, verify reset email received, click link, set new password, verify can sign in with new password
8. **Token expiry**: Wait 24h or manually set token expired, verify verification/reset links fail gracefully
9. **Resend verification**: After sign-up, click "Resend verification email", verify new email sent, verify rate-limiting works
10. **Pending badge**: When pending users exist, verify badge count shows on Users sidebar item