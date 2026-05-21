# Registration, Email & Sidebar Redesign — Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add email-verified registration with admin approval, SMTP email service, password reset, and clean up the dashboard sidebar navigation.

**Architecture:** Monolithic `EmailService` handles all SMTP operations. `EmailTokenService` manages one-time verification/reset tokens stored in a new `email_token` Ent schema. User status gains a `pending` state + `email_verified_at` field. Frontend gets new auth pages and two new System Settings tabs.

**Tech Stack:** Go (Ent ORM, gqlgen, Gin, net/smtp, html/template, embed.FS), React 19 + TypeScript (TanStack Router/Query, Zustand, Tailwind)

---

## File Structure

### New files — Backend

| File | Responsibility |
|------|---------------|
| `internal/ent/schema/email_token.go` | Ent schema for email verification/reset tokens |
| `internal/server/biz/email_token.go` | Create/validate/consume/cleanup token logic |
| `internal/server/biz/email.go` | SMTP email sending service |
| `internal/server/biz/email/templates/verify_email.html` | HTML email template for email verification |
| `internal/server/biz/email/templates/verify_email.txt` | Plain text variant |
| `internal/server/biz/email/templates/reset_password.html` | HTML email template for password reset |
| `internal/server/biz/email/templates/reset_password.txt` | Plain text variant |
| `internal/server/biz/email/templates/account_approved.html` | HTML email template for account approval |
| `internal/server/biz/email/templates/account_approved.txt` | Plain text variant |
| `internal/server/biz/email/templates/account_rejected.html` | HTML email template for account rejection |
| `internal/server/biz/email/templates/account_rejected.txt` | Plain text variant |
| `internal/server/biz/email/templates/admin_notification.html` | HTML email template for admin new-user notification |
| `internal/server/biz/email/templates/admin_notification.txt` | Plain text variant |
| `internal/server/api/email_token.go` | REST handlers: verify-email, forgot-password, reset-password, resend-verification |

### New files — Frontend

| File | Responsibility |
|------|---------------|
| `frontend/src/routes/(auth)/verify-email.tsx` | Email verification result page |
| `frontend/src/routes/(auth)/reset-password.tsx` | Password reset form page |
| `frontend/src/features/system/components/registration-settings.tsx` | Registration settings tab UI |
| `frontend/src/features/system/components/email-settings.tsx` | Email settings tab UI |

### Modified files — Backend

| File | Change |
|------|--------|
| `internal/ent/schema/user.go` | Add `pending` status value, `email_verified_at` field |
| `internal/server/biz/signup.go` | Create pending users, send verification email, remove auto-login |
| `internal/server/biz/user.go` | Add `ApproveUser`, `RejectUser` methods |
| `internal/server/biz/system.go` | Add `RegistrationSettings`, `EmailSettings` types + getters/setters |
| `internal/server/biz/gc.go` | Add expired email_token cleanup |
| `internal/server/biz/fx_module.go` | Register `EmailService`, `EmailTokenService` |
| `internal/server/api/fx_module.go` | Register `EmailTokenAPI` |
| `internal/server/routes.go` | Register new REST endpoints |
| `internal/server/gql/system.graphql` | Add RegistrationSettings, EmailSettings types + queries/mutations |
| `internal/server/gql/user.graphql` | Add `approveUser`, `rejectUser` mutations |
| `internal/server/gql/system.resolvers.go` | Add resolvers for new settings types |
| `internal/server/gql/user.resolvers.go` | Add resolvers for approve/reject |

### Modified files — Frontend

| File | Change |
|------|--------|
| `frontend/src/features/dashboard/index.tsx` | Remove Tabs toggle, hardcode mode per page |
| `frontend/src/features/dashboard/context.tsx` | Remove DashboardModeContext |
| `frontend/src/routes/_authenticated/index.tsx` | Remove DashboardModeContext.Provider |
| `frontend/src/sidebar.ts` | Add Users, Roles to admin sidebar group |
| `frontend/src/routes/_authenticated/users/index.tsx` | Update route path to /admin/users |
| `frontend/src/routes/_authenticated/roles/index.tsx` | Update route path to /admin/roles |
| `frontend/src/features/auth/sign-up/` | Remove auto-login after signup, show "check email" |
| `frontend/src/features/auth/forgot-password/` | Replace stub with real email-based flow |
| `frontend/src/features/auth/sign-in/` | Handle verified/pending URL params |
| `frontend/src/features/system/components/tabs.tsx` | Add Registration, Email tabs |
| `frontend/src/lib/api-client.ts` | Add email token REST API functions |
| `frontend/src/locales/en.json` | Add new i18n keys |
| `frontend/src/locales/zh.json` | Add new i18n keys |

---

## Task 1: Add `email_token` Ent Schema

**Files:**
- Create: `internal/ent/schema/email_token.go`

- [ ] **Step 1: Create the email_token schema**

```go
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"github.com/ldm2060/axonhub/internal/ent/schema/mixin"
)

type EmailToken struct {
	ent.Schema
}

func (EmailToken) Mixin() []ent.Mixin {
	return []ent.Mixin{
		mixin.Time{},
	}
}

func (EmailToken) Fields() []ent.Field {
	return []ent.Field{
		field.String("token").Unique().NotEmpty().Comment("Random UUID token"),
		field.Enum("type").Values("verify_email", "reset_password").Comment("Token type"),
		field.Time("expires_at").Comment("Token expiration time"),
		field.Time("consumed_at").Optional().Nillable().Comment("When the token was consumed"),
	}
}

func (EmailToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).Ref("email_tokens").Unique().Required().Annotations(entsql.OnDelete(entsql.Cascade)),
	}
}

func (EmailToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("token"),
		index.Fields("type", "expires_at"),
	}
}

func (EmailToken) Annotations() []ent.Annotation {
	return []ent.Annotation{}
}
```

- [ ] **Step 2: Add reverse edge to user schema**

In `internal/ent/schema/user.go`, add to the `Edges()` function:

```go
edge.To("email_tokens", EmailToken.Type).Annotations(entsql.OnDelete(entsql.Cascade)),
```

- [ ] **Step 3: Run code generation**

Run: `cd D:\PythonProject\axonhub && go generate ./internal/ent`

Expected: No errors, new files generated under `internal/ent/emailtoken/`, `internal/ent/emailtoken/where/`, etc.

- [ ] **Step 4: Commit**

```bash
git add internal/ent/schema/email_token.go internal/ent/schema/user.go internal/ent/
git commit -m "feat: add email_token Ent schema for verification and password reset"
```

---

## Task 2: Update User Schema — Add `pending` Status + `email_verified_at`

**Files:**
- Modify: `internal/ent/schema/user.go`

- [ ] **Step 1: Add `pending` to the status enum**

In `internal/ent/schema/user.go`, find the `status` field definition (currently has values like `"activated", "deactivated"`). Add `"pending"` as the first value:

```go
field.Enum("status").
	Default("pending").
	Values("pending", "activated", "deactivated").
	Comment("User account status"),
```

Change the default from `"activated"` to `"pending"` — new users start as pending.

- [ ] **Step 2: Add `email_verified_at` field**

Add to the `Fields()` function:

```go
field.Time("email_verified_at").Optional().Nillable().Comment("When the user verified their email"),
```

- [ ] **Step 3: Run code generation**

Run: `cd D:\PythonProject\axonhub && go generate ./internal/ent`

Expected: No errors. The generated code now includes `pending` status and `email_verified_at` field.

- [ ] **Step 4: Commit**

```bash
git add internal/ent/schema/user.go internal/ent/
git commit -m "feat: add pending status and email_verified_at to user schema"
```

---

## Task 3: Add RegistrationSettings + EmailSettings to System Service

**Files:**
- Modify: `internal/server/biz/system.go`

- [ ] **Step 1: Add RegistrationSettings type and CRUD**

Find the pattern for existing settings types in `system.go` (e.g., how `GeneralSettings` or `WebhookSettings` are defined). Add:

```go
type RegistrationSettings struct {
	AllowSignUp       bool     `json:"allow_sign_up"`
	ApprovalRequired  bool     `json:"approval_required"`
	DefaultUserScopes []string `json:"default_user_scopes"`
}

func (s *SystemService) RegistrationSettings(ctx context.Context) (*RegistrationSettings, error) {
	v, err := s.getSetting(ctx, "registration_settings")
	if err != nil {
		return nil, err
	}
	rs := &RegistrationSettings{AllowSignUp: false, ApprovalRequired: false}
	if v != "" {
		if err := xjson.Unmarshal(v, rs); err != nil {
			return nil, err
		}
	}
	return rs, nil
}

func (s *SystemService) UpdateRegistrationSettings(ctx context.Context, input *RegistrationSettings) error {
	return s.updateSetting(ctx, "registration_settings", xjson.MustMarshal(input))
}
```

- [ ] **Step 2: Add EmailSettings type and CRUD**

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

func (s *SystemService) EmailSettings(ctx context.Context) (*EmailSettings, error) {
	v, err := s.getSetting(ctx, "email_settings")
	if err != nil {
		return nil, err
	}
	es := &EmailSettings{}
	if v != "" {
		if err := xjson.Unmarshal(v, es); err != nil {
			return nil, err
		}
	}
	return es, nil
}

func (s *SystemService) UpdateEmailSettings(ctx context.Context, input *EmailSettings) error {
	return s.updateSetting(ctx, "email_settings", xjson.MustMarshal(input))
}
```

- [ ] **Step 3: Add migration for old settings keys**

In the `RegistrationSettings` getter, add fallback logic to read the old separate keys if the consolidated key doesn't exist yet:

```go
func (s *SystemService) RegistrationSettings(ctx context.Context) (*RegistrationSettings, error) {
	v, err := s.getSetting(ctx, "registration_settings")
	if err != nil {
		return nil, err
	}
	rs := &RegistrationSettings{AllowSignUp: false, ApprovalRequired: false}
	if v != "" {
		if err := xjson.Unmarshal(v, rs); err != nil {
			return nil, err
		}
	} else {
		// Migrate from old separate keys
		allowSignUp, _ := s.getSetting(ctx, "allow_sign_up")
		approvalRequired, _ := s.getSetting(ctx, "sign_up_approval_required")
		rs.AllowSignUp = allowSignUp == "true"
		rs.ApprovalRequired = approvalRequired == "true"
		// Persist consolidated key
		_ = s.updateSetting(ctx, "registration_settings", xjson.MustMarshal(rs))
	}
	return rs, nil
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/server/biz/system.go
git commit -m "feat: add RegistrationSettings and EmailSettings to SystemService"
```

---

## Task 4: Create EmailTokenService

**Files:**
- Create: `internal/server/biz/email_token.go`
- Modify: `internal/server/biz/fx_module.go`

- [ ] **Step 1: Create the EmailTokenService**

```go
package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/ldm2060/axonhub/internal/ent"
	entEmailToken "github.com/ldm2060/axonhub/internal/ent/emailtoken"
	"github.com/ldm2060/axonhub/internal/pkg/xcontext"
)

type EmailTokenService struct {
	abstractService
}

func NewEmailTokenService() *EmailTokenService {
	return &EmailTokenService{}
}

func (s *EmailTokenService) CreateToken(ctx context.Context, userID int, tokenType entEmailToken.Type) (string, error) {
	token := uuid.New().String()
	_, err := s.entClient.EmailToken.Create().
		SetToken(token).
		SetType(tokenType).
		SetExpiresAt(time.Now().Add(24 * time.Hour)).
		SetUserID(userID).
		Save(ctx)
	if err != nil {
		return "", fmt.Errorf("create email token: %w", err)
	}
	return token, nil
}

func (s *EmailTokenService) ValidateToken(ctx context.Context, token string, tokenType entEmailToken.Type) (int, error) {
	t, err := s.entClient.EmailToken.Query().
		Where(
			entEmailToken.Token(token),
			entEmailToken.Type(tokenType),
			entEmailToken.ExpiresAtGT(time.Now()),
			entEmailToken.ConsumedAtIsNil(),
		).
		Only(ctx)
	if err != nil {
		return 0, fmt.Errorf("validate email token: %w", err)
	}
	return t.UserID, nil
}

func (s *EmailTokenService) ConsumeToken(ctx context.Context, token string) error {
	_, err := s.entClient.EmailToken.Update().
		Where(entEmailToken.Token(token)).
		SetConsumedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("consume email token: %w", err)
	}
	return nil
}

func (s *EmailTokenService) CleanupExpired(ctx context.Context) error {
	_, err := s.entClient.EmailToken.Delete().
		Where(entEmailToken.ExpiresAtLT(time.Now())).
		Exec(ctx)
	if err != nil {
		return fmt.Errorf("cleanup expired email tokens: %w", err)
	}
	return nil
}
```

- [ ] **Step 2: Register in FX module**

In `internal/server/biz/fx_module.go`, add to the `Provide` list:

```go
fx.Provide(NewEmailTokenService),
```

Wire the `entClient` through the same pattern as other services (via the `abstractService` embed which receives it from FX).

- [ ] **Step 3: Commit**

```bash
git add internal/server/biz/email_token.go internal/server/biz/fx_module.go
git commit -m "feat: add EmailTokenService for verification and reset tokens"
```

---

## Task 5: Create EmailService

**Files:**
- Create: `internal/server/biz/email.go`
- Create: `internal/server/biz/email/templates/` (all template files)
- Modify: `internal/server/biz/fx_module.go`

- [ ] **Step 1: Create the email templates directory**

Create `internal/server/biz/email/templates/` directory.

- [ ] **Step 2: Create verify_email.html template**

```html
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 0; background-color: #f8fafc;">
<div style="max-width: 600px; margin: 40px auto; background: white; border-radius: 8px; overflow: hidden; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
  <div style="background: #3b82f6; padding: 24px; text-align: center;">
    <h1 style="color: white; margin: 0; font-size: 20px;">{{.BrandName}}</h1>
  </div>
  <div style="padding: 32px;">
    <h2 style="color: #0f172a; font-size: 18px; margin: 0 0 16px;">Verify Your Email Address</h2>
    <p style="color: #475569; font-size: 14px; line-height: 1.6;">Hello {{.RecipientName}},</p>
    <p style="color: #475569; font-size: 14px; line-height: 1.6;">Please click the button below to verify your email address. This link will expire in 24 hours.</p>
    <div style="text-align: center; margin: 24px 0;">
      <a href="{{.ActionURL}}" style="background: #3b82f6; color: white; padding: 12px 32px; border-radius: 6px; text-decoration: none; font-size: 14px; font-weight: 500; display: inline-block;">Verify Email</a>
    </div>
    <p style="color: #94a3b8; font-size: 12px; line-height: 1.6;">If the button doesn't work, copy and paste this link into your browser:<br>{{.ActionURL}}</p>
  </div>
  <div style="background: #f8fafc; padding: 16px; text-align: center; color: #94a3b8; font-size: 12px;">
    This email was sent by {{.BrandName}}. If you didn't create an account, you can ignore this email.
  </div>
</div>
</body>
</html>
```

- [ ] **Step 3: Create verify_email.txt template**

```
{{.BrandName}} — Verify Your Email Address

Hello {{.RecipientName}},

Please click the link below to verify your email address. This link will expire in 24 hours.

{{.ActionURL}}

If you didn't create an account, you can ignore this email.
```

- [ ] **Step 4: Create reset_password.html template**

```html
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 0; background-color: #f8fafc;">
<div style="max-width: 600px; margin: 40px auto; background: white; border-radius: 8px; overflow: hidden; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
  <div style="background: #3b82f6; padding: 24px; text-align: center;">
    <h1 style="color: white; margin: 0; font-size: 20px;">{{.BrandName}}</h1>
  </div>
  <div style="padding: 32px;">
    <h2 style="color: #0f172a; font-size: 18px; margin: 0 0 16px;">Reset Your Password</h2>
    <p style="color: #475569; font-size: 14px; line-height: 1.6;">Hello {{.RecipientName}},</p>
    <p style="color: #475569; font-size: 14px; line-height: 1.6;">We received a request to reset your password. Click the button below to choose a new one. This link will expire in 24 hours.</p>
    <div style="text-align: center; margin: 24px 0;">
      <a href="{{.ActionURL}}" style="background: #3b82f6; color: white; padding: 12px 32px; border-radius: 6px; text-decoration: none; font-size: 14px; font-weight: 500; display: inline-block;">Reset Password</a>
    </div>
    <p style="color: #94a3b8; font-size: 12px; line-height: 1.6;">If the button doesn't work, copy and paste this link into your browser:<br>{{.ActionURL}}</p>
  </div>
  <div style="background: #f8fafc; padding: 16px; text-align: center; color: #94a3b8; font-size: 12px;">
    If you didn't request a password reset, you can safely ignore this email.
  </div>
</div>
</body>
</html>
```

- [ ] **Step 5: Create reset_password.txt template**

```
{{.BrandName}} — Reset Your Password

Hello {{.RecipientName}},

We received a request to reset your password. Click the link below to choose a new one. This link will expire in 24 hours.

{{.ActionURL}}

If you didn't request a password reset, you can safely ignore this email.
```

- [ ] **Step 6: Create account_approved.html template**

```html
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 0; background-color: #f8fafc;">
<div style="max-width: 600px; margin: 40px auto; background: white; border-radius: 8px; overflow: hidden; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
  <div style="background: #22c55e; padding: 24px; text-align: center;">
    <h1 style="color: white; margin: 0; font-size: 20px;">{{.BrandName}}</h1>
  </div>
  <div style="padding: 32px;">
    <h2 style="color: #0f172a; font-size: 18px; margin: 0 0 16px;">Your Account Has Been Approved</h2>
    <p style="color: #475569; font-size: 14px; line-height: 1.6;">Hello {{.RecipientName}},</p>
    <p style="color: #475569; font-size: 14px; line-height: 1.6;">Great news! Your account has been approved by an administrator. You can now sign in and start using the platform.</p>
    <div style="text-align: center; margin: 24px 0;">
      <a href="{{.ActionURL}}" style="background: #22c55e; color: white; padding: 12px 32px; border-radius: 6px; text-decoration: none; font-size: 14px; font-weight: 500; display: inline-block;">Sign In</a>
    </div>
  </div>
  <div style="background: #f8fafc; padding: 16px; text-align: center; color: #94a3b8; font-size: 12px;">
    This email was sent by {{.BrandName}}.
  </div>
</div>
</body>
</html>
```

- [ ] **Step 7: Create account_approved.txt template**

```
{{.BrandName}} — Your Account Has Been Approved

Hello {{.RecipientName}},

Great news! Your account has been approved by an administrator. You can now sign in and start using the platform.

Sign in: {{.ActionURL}}
```

- [ ] **Step 8: Create account_rejected.html template**

```html
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 0; background-color: #f8fafc;">
<div style="max-width: 600px; margin: 40px auto; background: white; border-radius: 8px; overflow: hidden; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
  <div style="background: #64748b; padding: 24px; text-align: center;">
    <h1 style="color: white; margin: 0; font-size: 20px;">{{.BrandName}}</h1>
  </div>
  <div style="padding: 32px;">
    <h2 style="color: #0f172a; font-size: 18px; margin: 0 0 16px;">Registration Update</h2>
    <p style="color: #475569; font-size: 14px; line-height: 1.6;">Hello {{.RecipientName}},</p>
    <p style="color: #475569; font-size: 14px; line-height: 1.6;">We're sorry, but your registration request was not approved by an administrator. If you believe this is an error, please contact your system administrator.</p>
  </div>
  <div style="background: #f8fafc; padding: 16px; text-align: center; color: #94a3b8; font-size: 12px;">
    This email was sent by {{.BrandName}}.
  </div>
</div>
</body>
</html>
```

- [ ] **Step 9: Create account_rejected.txt template**

```
{{.BrandName}} — Registration Update

Hello {{.RecipientName}},

We're sorry, but your registration request was not approved by an administrator. If you believe this is an error, please contact your system administrator.
```

- [ ] **Step 10: Create admin_notification.html template**

```html
<!DOCTYPE html>
<html>
<head><meta charset="utf-8"></head>
<body style="font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; margin: 0; padding: 0; background-color: #f8fafc;">
<div style="max-width: 600px; margin: 40px auto; background: white; border-radius: 8px; overflow: hidden; box-shadow: 0 1px 3px rgba(0,0,0,0.1);">
  <div style="background: #f59e0b; padding: 24px; text-align: center;">
    <h1 style="color: white; margin: 0; font-size: 20px;">{{.BrandName}}</h1>
  </div>
  <div style="padding: 32px;">
    <h2 style="color: #0f172a; font-size: 18px; margin: 0 0 16px;">New User Registration Requires Approval</h2>
    <p style="color: #475569; font-size: 14px; line-height: 1.6;">A new user has registered and verified their email address:</p>
    <div style="background: #f8fafc; border: 1px solid #e2e8f0; border-radius: 6px; padding: 16px; margin: 16px 0;">
      <p style="margin: 0 0 8px; color: #0f172a; font-size: 14px;"><strong>Name:</strong> {{.RecipientName}}</p>
      <p style="margin: 0; color: #0f172a; font-size: 14px;"><strong>Email:</strong> {{.Extra}}</p>
    </div>
    <div style="text-align: center; margin: 24px 0;">
      <a href="{{.ActionURL}}" style="background: #f59e0b; color: white; padding: 12px 32px; border-radius: 6px; text-decoration: none; font-size: 14px; font-weight: 500; display: inline-block;">Review Users</a>
    </div>
  </div>
  <div style="background: #f8fafc; padding: 16px; text-align: center; color: #94a3b8; font-size: 12px;">
    This is an automated notification from {{.BrandName}}.
  </div>
</div>
</body>
</html>
```

- [ ] **Step 11: Create admin_notification.txt template**

```
{{.BrandName}} — New User Registration Requires Approval

A new user has registered and verified their email address:

Name: {{.RecipientName}}
Email: {{.Extra}}

Review pending users: {{.ActionURL}}
```

- [ ] **Step 12: Create the EmailService**

```go
package biz

import (
	"bytes"
	"context"
	"crypto/tls"
	"embed"
	"fmt"
	"html/template"
	"net/smtp"
	"strings"

	"github.com/ldm2060/axonhub/internal/pkg/xcontext"
)

//go:embed email/templates/*.html email/templates/*.txt
var templateFS embed.FS

type emailTemplateData struct {
	BrandName     string
	BrandLogoURL  string
	RecipientName string
	ActionURL     string
	Extra         string
}

type EmailService struct {
	abstractService
	systemService *SystemService
	htmlTemplates *template.Template
	textTemplates *template.Template
}

func NewEmailService(systemService *SystemService) *EmailService {
	htmlTmpl := template.Must(template.New("").ParseFS(templateFS, "email/templates/*.html"))
	textTmpl := template.Must(template.New("").ParseFS(templateFS, "email/templates/*.txt"))
	return &EmailService{
		systemService: systemService,
		htmlTemplates: htmlTmpl,
		textTemplates: textTmpl,
	}
}

func (s *EmailService) Send(ctx context.Context, to, subject, htmlBody, textBody string) error {
	settings, err := s.systemService.EmailSettings(ctx)
	if err != nil {
		return fmt.Errorf("get email settings: %w", err)
	}
	if settings.SMTPHost == "" {
		return fmt.Errorf("email service not configured")
	}

	from := settings.FromAddress
	if settings.FromName != "" {
		from = fmt.Sprintf("%s <%s>", settings.FromName, settings.FromAddress)
	}

	var msg bytes.Buffer
	msg.WriteString("From: " + from + "\r\n")
	msg.WriteString("To: " + to + "\r\n")
	msg.WriteString("Subject: " + subject + "\r\n")
	msg.WriteString("MIME-Version: 1.0\r\n")
	msg.WriteString("Content-Type: multipart/alternative; boundary=\"boundary\"\r\n\r\n")
	msg.WriteString("--boundary\r\n")
	msg.WriteString("Content-Type: text/plain; charset=utf-8\r\n\r\n")
	msg.WriteString(textBody + "\r\n\r\n")
	msg.WriteString("--boundary\r\n")
	msg.WriteString("Content-Type: text/html; charset=utf-8\r\n\r\n")
	msg.WriteString(htmlBody + "\r\n\r\n")
	msg.WriteString("--boundary--")

	addr := fmt.Sprintf("%s:%d", settings.SMTPHost, settings.SMTPPort)
	auth := smtp.PlainAuth("", settings.SMTPUser, settings.SMTPPassword, settings.SMTPHost)

	switch settings.Encryption {
	case "ssl":
		tlsConfig := &tls.Config{ServerName: settings.SMTPHost}
		conn, err := tls.Dial("tcp", addr, tlsConfig)
		if err != nil {
			return fmt.Errorf("smtp tls dial: %w", err)
		}
		defer conn.Close()
		c, err := smtp.NewClient(conn, settings.SMTPHost)
		if err != nil {
			return fmt.Errorf("smtp client: %w", err)
		}
		defer c.Close()
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
		if err := c.Mail(settings.FromAddress); err != nil {
			return fmt.Errorf("smtp mail: %w", err)
		}
		if err := c.Rcpt(to); err != nil {
			return fmt.Errorf("smtp rcpt: %w", err)
		}
		w, err := c.Data()
		if err != nil {
			return fmt.Errorf("smtp data: %w", err)
		}
		if _, err := w.Write(msg.Bytes()); err != nil {
			return fmt.Errorf("smtp write: %w", err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("smtp close data: %w", err)
		}
		return c.Quit()
	default: // "starttls" or "none"
		c, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("smtp dial: %w", err)
		}
		defer c.Close()
		if settings.Encryption == "starttls" {
			if err := c.StartTLS(&tls.Config{ServerName: settings.SMTPHost}); err != nil {
				return fmt.Errorf("smtp starttls: %w", err)
			}
		}
		if err := c.Auth(auth); err != nil {
			return fmt.Errorf("smtp auth: %w", err)
		}
		if err := c.Mail(settings.FromAddress); err != nil {
			return fmt.Errorf("smtp mail: %w", err)
		}
		if err := c.Rcpt(to); err != nil {
			return fmt.Errorf("smtp rcpt: %w", err)
		}
		w, err := c.Data()
		if err != nil {
			return fmt.Errorf("smtp data: %w", err)
		}
		if _, err := w.Write(msg.Bytes()); err != nil {
			return fmt.Errorf("smtp write: %w", err)
		}
		if err := w.Close(); err != nil {
			return fmt.Errorf("smtp close data: %w", err)
		}
		return c.Quit()
	}
}

func (s *EmailService) renderTemplate(name string, data *emailTemplateData) (string, string, error) {
	var htmlBuf, textBuf bytes.Buffer
	htmlName := "email/templates/" + name + ".html"
	textName := "email/templates/" + name + ".txt"

	if err := s.htmlTemplates.ExecuteTemplate(&htmlBuf, htmlName, data); err != nil {
		return "", "", fmt.Errorf("render html template %s: %w", name, err)
	}
	if err := s.textTemplates.ExecuteTemplate(&textBuf, textName, data); err != nil {
		return "", "", fmt.Errorf("render text template %s: %w", name, err)
	}
	return htmlBuf.String(), textBuf.String(), nil
}

func (s *EmailService) brandName(ctx context.Context) string {
	gs, err := s.systemService.GeneralSettings(ctx)
	if err != nil || gs.BrandName == "" {
		return "AxonHub"
	}
	return gs.BrandName
}

func (s *EmailService) SendVerificationEmail(ctx context.Context, userEmail, userName, tokenURL string) error {
	data := &emailTemplateData{
		BrandName:     s.brandName(ctx),
		RecipientName: userName,
		ActionURL:     tokenURL,
	}
	htmlBody, textBody, err := s.renderTemplate("verify_email", data)
	if err != nil {
		return err
	}
	return s.Send(ctx, userEmail, s.brandName(ctx)+" — Verify Your Email", htmlBody, textBody)
}

func (s *EmailService) SendPasswordResetEmail(ctx context.Context, userEmail, userName, tokenURL string) error {
	data := &emailTemplateData{
		BrandName:     s.brandName(ctx),
		RecipientName: userName,
		ActionURL:     tokenURL,
	}
	htmlBody, textBody, err := s.renderTemplate("reset_password", data)
	if err != nil {
		return err
	}
	return s.Send(ctx, userEmail, s.brandName(ctx)+" — Reset Your Password", htmlBody, textBody)
}

func (s *EmailService) SendApprovedEmail(ctx context.Context, userEmail, userName, signInURL string) error {
	data := &emailTemplateData{
		BrandName:     s.brandName(ctx),
		RecipientName: userName,
		ActionURL:     signInURL,
	}
	htmlBody, textBody, err := s.renderTemplate("account_approved", data)
	if err != nil {
		return err
	}
	return s.Send(ctx, userEmail, s.brandName(ctx)+" — Account Approved", htmlBody, textBody)
}

func (s *EmailService) SendRejectedEmail(ctx context.Context, userEmail, userName string) error {
	data := &emailTemplateData{
		BrandName:     s.brandName(ctx),
		RecipientName: userName,
	}
	htmlBody, textBody, err := s.renderTemplate("account_rejected", data)
	if err != nil {
		return err
	}
	return s.Send(ctx, userEmail, s.brandName(ctx)+" — Registration Update", htmlBody, textBody)
}

func (s *EmailService) SendAdminNotification(ctx context.Context, adminEmail, userName, userEmail, reviewURL string) error {
	data := &emailTemplateData{
		BrandName:     s.brandName(ctx),
		RecipientName: userName,
		ActionURL:     reviewURL,
		Extra:         userEmail,
	}
	htmlBody, textBody, err := s.renderTemplate("admin_notification", data)
	if err != nil {
		return err
	}
	return s.Send(ctx, adminEmail, s.brandName(ctx)+" — New User Requires Approval", htmlBody, textBody)
}

func (s *EmailService) SendTestEmail(ctx context.Context, to string) error {
	data := &emailTemplateData{
		BrandName:     s.brandName(ctx),
		RecipientName: to,
	}
	htmlBody := fmt.Sprintf(`<div style="font-family:sans-serif;padding:32px;"><h2>Email Test from %s</h2><p>If you're seeing this, your SMTP configuration is working correctly.</p></div>`, data.BrandName)
	textBody := fmt.Sprintf("Email Test from %s\n\nIf you're seeing this, your SMTP configuration is working correctly.", data.BrandName)
	return s.Send(ctx, to, s.brandName(ctx)+" — Test Email", htmlBody, textBody)
}

func (s *EmailService) TestConnection(ctx context.Context) error {
	settings, err := s.systemService.EmailSettings(ctx)
	if err != nil {
		return fmt.Errorf("get email settings: %w", err)
	}
	if settings.SMTPHost == "" {
		return fmt.Errorf("SMTP host not configured")
	}
	addr := fmt.Sprintf("%s:%d", settings.SMTPHost, settings.SMTPPort)
	switch settings.Encryption {
	case "ssl":
		conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: settings.SMTPHost})
		if err != nil {
			return fmt.Errorf("connect failed: %w", err)
		}
		conn.Close()
	default:
		c, err := smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("connect failed: %w", err)
		}
		c.Close()
	}
	return nil
}
```

- [ ] **Step 13: Register in FX module**

In `internal/server/biz/fx_module.go`, add to the `Provide` list:

```go
fx.Provide(NewEmailService),
```

- [ ] **Step 14: Commit**

```bash
git add internal/server/biz/email.go internal/server/biz/email/ internal/server/biz/fx_module.go
git commit -m "feat: add EmailService with SMTP sending and HTML/text templates"
```

---

## Task 6: Add Email Token Cleanup to GC Service

**Files:**
- Modify: `internal/server/biz/gc.go`

- [ ] **Step 1: Add EmailTokenService dependency to GC**

Find the GC service struct and add the `emailTokenService` field:

```go
type GCService struct {
	abstractService
	// ... existing fields ...
	emailTokenService *EmailTokenService
}
```

Update the constructor to accept `*EmailTokenService` as a parameter.

- [ ] **Step 2: Add cleanup call to the GC tick function**

In the GC tick/run function, add a call to cleanup expired email tokens:

```go
if err := s.emailTokenService.CleanupExpired(ctx); err != nil {
	s.logger.Error("cleanup expired email tokens", zap.Error(err))
}
```

- [ ] **Step 3: Commit**

```bash
git add internal/server/biz/gc.go
git commit -m "feat: add expired email token cleanup to GC service"
```

---

## Task 7: Update SignUpService

**Files:**
- Modify: `internal/server/biz/signup.go`

- [ ] **Step 1: Add EmailService and EmailTokenService dependencies**

Add fields to the SignUpService struct:

```go
emailService      *EmailService
emailTokenService *EmailTokenService
```

Update the constructor to accept both.

- [ ] **Step 2: Update SignUp method — create pending user, send verification email**

The current `SignUp` method creates an `activated` user and returns a session token. Change it to:

1. Create user with `status=pending` and `email_verified_at=nil`
2. Create an email verification token via `EmailTokenService`
3. Send verification email via `EmailService`
4. Return the user WITHOUT a session token (the caller should NOT auto-login)

Key changes to the SignUp method body:

```go
// Replace the status assignment
userCreate := s.entClient.User.Create().
	SetFirstName(input.FirstName).
	SetLastName(input.LastName).
	SetEmail(input.Email).
	SetPassword(hashedPassword).
	SetStatus(user.StatusPending). // was: StatusActivated
	// ... rest of fields ...

u, err := userCreate.Save(ctx)
// ... error handling ...

// Create verification token
token, err := s.emailTokenService.CreateToken(ctx, u.ID, entEmailToken.TypeVerifyEmail)
// ... error handling ...

// Build verification URL
verifyURL := fmt.Sprintf("%s/admin/auth/verify-email?token=%s", s.baseURL(ctx), token)

// Send verification email (non-blocking — log error but don't fail signup)
if err := s.emailService.SendVerificationEmail(ctx, u.Email, u.FirstName+" "+u.LastName, verifyURL); err != nil {
	s.logger.Warn("failed to send verification email", zap.Error(err))
}

return u, nil
```

Remove the auto-login / session token creation logic from the SignUp method.

- [ ] **Step 3: Update the signup REST handler**

In the signup API handler, change the response to NOT include a session token. Instead return a simple success message:

```go
c.JSON(http.StatusOK, gin.H{"message": "Registration successful. Please check your email to verify your account."})
```

- [ ] **Step 4: Commit**

```bash
git add internal/server/biz/signup.go
git commit -m "feat: signup creates pending user and sends verification email"
```

---

## Task 8: Add ApproveUser / RejectUser to UserService

**Files:**
- Modify: `internal/server/biz/user.go`

- [ ] **Step 1: Add EmailService dependency**

Add `emailService *EmailService` field to UserService struct and update the constructor.

- [ ] **Step 2: Add ApproveUser method**

```go
func (s *UserService) ApproveUser(ctx context.Context, userID int) error {
	u, err := s.entClient.User.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if u.Status != user.StatusPending {
		return fmt.Errorf("user is not pending approval")
	}

	// Get default scopes from registration settings
	rs, err := s.systemService.RegistrationSettings(ctx)
	if err != nil {
		return fmt.Errorf("get registration settings: %w", err)
	}

	// Update user status and assign scopes
	update := s.entClient.User.UpdateOneID(userID).
		SetStatus(user.StatusActivated).
		SetScopes(rs.DefaultUserScopes)
	_, err = update.Save(ctx)
	if err != nil {
		return fmt.Errorf("approve user: %w", err)
	}

	// Send approval email (non-blocking)
	signInURL := s.baseURL(ctx) + "/sign-in"
	if err := s.emailService.SendApprovedEmail(ctx, u.Email, u.FirstName+" "+u.LastName, signInURL); err != nil {
		s.logger.Warn("failed to send approval email", zap.Error(err))
	}

	return nil
}
```

- [ ] **Step 3: Add RejectUser method**

```go
func (s *UserService) RejectUser(ctx context.Context, userID int) error {
	u, err := s.entClient.User.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if u.Status != user.StatusPending {
		return fmt.Errorf("user is not pending approval")
	}

	// Send rejection email before deleting (non-blocking)
	if err := s.emailService.SendRejectedEmail(ctx, u.Email, u.FirstName+" "+u.LastName); err != nil {
		s.logger.Warn("failed to send rejection email", zap.Error(err))
	}

	// Delete the user record
	err = s.entClient.User.DeleteOneID(userID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	return nil
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/server/biz/user.go
git commit -m "feat: add ApproveUser and RejectUser methods to UserService"
```

---

## Task 9: Create Email Token REST Handlers

**Files:**
- Create: `internal/server/api/email_token.go`
- Modify: `internal/server/api/fx_module.go`
- Modify: `internal/server/routes.go`

- [ ] **Step 1: Create the REST handlers**

```go
package api

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	entEmailToken "github.com/ldm2060/axonhub/internal/ent/emailtoken"
	"github.com/ldm2060/axonhub/internal/ent/user"
	"github.com/ldm2060/axonhub/internal/pkg/xerrors"
	"github.com/ldm2060/axonhub/internal/server/biz"
)

type EmailTokenAPI struct {
	emailTokenService *biz.EmailTokenService
	emailService      *biz.EmailService
	userService       *biz.UserService
	systemService     *biz.SystemService
	rateLimiter       map[string]time.Time
	rateMu            sync.Mutex
}

func NewEmailTokenAPI(
	emailTokenService *biz.EmailTokenService,
	emailService *biz.EmailService,
	userService *biz.UserService,
	systemService *biz.SystemService,
) *EmailTokenAPI {
	return &EmailTokenAPI{
		emailTokenService: emailTokenService,
		emailService:      emailService,
		userService:       userService,
		systemService:     systemService,
		rateLimiter:       make(map[string]time.Time),
	}
}

func (a *EmailTokenAPI) VerifyEmail(c *gin.Context) {
	token := c.Query("token")
	if token == "" {
		c.Redirect(http.StatusFound, "/sign-in?verified=0")
		return
	}

	userID, err := a.emailTokenService.ValidateToken(c.Request.Context(), token, entEmailToken.TypeVerifyEmail)
	if err != nil {
		c.Redirect(http.StatusFound, "/sign-in?verified=0")
		return
	}

	if err := a.emailTokenService.ConsumeToken(c.Request.Context(), token); err != nil {
		c.Redirect(http.StatusFound, "/sign-in?verified=0")
		return
	}

	// Mark email as verified
	if err := a.userService.MarkEmailVerified(c.Request.Context(), userID); err != nil {
		c.Redirect(http.StatusFound, "/sign-in?verified=0")
		return
	}

	// Check if approval is required
	rs, err := a.systemService.RegistrationSettings(c.Request.Context())
	if err != nil {
		c.Redirect(http.StatusFound, "/sign-in?verified=1")
		return
	}

	if rs.ApprovalRequired {
		// Notify admins about pending user
		a.notifyAdmins(c, userID)
		c.Redirect(http.StatusFound, "/sign-in?verified=1&pending=1")
		return
	}

	// Auto-approve
	if err := a.userService.ActivateUser(c.Request.Context(), userID); err != nil {
		c.Redirect(http.StatusFound, "/sign-in?verified=1")
		return
	}

	c.Redirect(http.StatusFound, "/sign-in?verified=1")
}

func (a *EmailTokenAPI) ResendVerification(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		xerrors.Response(c, xerrors.BadRequest(err.Error()))
		return
	}

	// Rate limit: 1 per minute per email
	if !a.checkRateLimit(req.Email) {
		xerrors.Response(c, xerrors.TooManyRequests("Please wait before requesting another email"))
		return
	}

	u, err := a.userService.GetByEmail(c.Request.Context(), req.Email)
	if err != nil || u.Status != user.StatusPending || u.EmailVerifiedAt != nil {
		// Don't reveal whether the email exists — return success anyway
		c.JSON(http.StatusOK, gin.H{"message": "If the email exists and needs verification, a new email has been sent."})
		return
	}

	token, err := a.emailTokenService.CreateToken(c.Request.Context(), u.ID, entEmailToken.TypeVerifyEmail)
	if err != nil {
		xerrors.Response(c, xerrors.Internal(err.Error()))
		return
	}

	verifyURL := a.baseURL(c) + "/admin/auth/verify-email?token=" + token
	if err := a.emailService.SendVerificationEmail(c.Request.Context(), u.Email, u.FirstName+" "+u.LastName, verifyURL); err != nil {
		xerrors.Response(c, xerrors.Internal("Failed to send email"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "If the email exists and needs verification, a new email has been sent."})
}

func (a *EmailTokenAPI) ForgotPassword(c *gin.Context) {
	var req struct {
		Email string `json:"email" binding:"required,email"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		xerrors.Response(c, xerrors.BadRequest(err.Error()))
		return
	}

	if !a.checkRateLimit(req.Email) {
		xerrors.Response(c, xerrors.TooManyRequests("Please wait before requesting another email"))
		return
	}

	u, err := a.userService.GetByEmail(c.Request.Context(), req.Email)
	if err != nil || u.Status != user.StatusActivated {
		// Don't reveal whether the email exists
		c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a reset link has been sent."})
		return
	}

	token, err := a.emailTokenService.CreateToken(c.Request.Context(), u.ID, entEmailToken.TypeResetPassword)
	if err != nil {
		xerrors.Response(c, xerrors.Internal(err.Error()))
		return
	}

	resetURL := a.baseURL(c) + "/reset-password?token=" + token
	if err := a.emailService.SendPasswordResetEmail(c.Request.Context(), u.Email, u.FirstName+" "+u.LastName, resetURL); err != nil {
		xerrors.Response(c, xerrors.Internal("Failed to send email"))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "If the email exists, a reset link has been sent."})
}

func (a *EmailTokenAPI) ResetPassword(c *gin.Context) {
	var req struct {
		Token    string `json:"token" binding:"required"`
		Password string `json:"password" binding:"required,min=8"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		xerrors.Response(c, xerrors.BadRequest(err.Error()))
		return
	}

	userID, err := a.emailTokenService.ValidateToken(c.Request.Context(), req.Token, entEmailToken.TypeResetPassword)
	if err != nil {
		xerrors.Response(c, xerrors.BadRequest("Invalid or expired reset token"))
		return
	}

	if err := a.emailTokenService.ConsumeToken(c.Request.Context(), req.Token); err != nil {
		xerrors.Response(c, xerrors.Internal(err.Error()))
		return
	}

	if err := a.userService.ResetPassword(c.Request.Context(), userID, req.Password); err != nil {
		xerrors.Response(c, xerrors.Internal(err.Error()))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Password has been reset successfully."})
}

func (a *EmailTokenAPI) checkRateLimit(email string) bool {
	a.rateMu.Lock()
	defer a.rateMu.Unlock()
	if last, ok := a.rateLimiter[email]; ok && time.Since(last) < time.Minute {
		return false
	}
	a.rateLimiter[email] = time.Now()
	return true
}

func (a *EmailTokenAPI) notifyAdmins(c *gin.Context, userID int) {
	// Query all owner-role users and send notification
	// This is a best-effort operation — errors are logged but don't fail the request
	admins, err := a.userService.ListOwners(c.Request.Context())
	if err != nil {
		return
	}
	u, err := a.userService.Get(c.Request.Context(), userID)
	if err != nil {
		return
	}
	reviewURL := a.baseURL(c) + "/admin/users"
	for _, admin := range admins {
		if err := a.emailService.SendAdminNotification(c.Request.Context(), admin.Email, u.FirstName+" "+u.LastName, u.Email, reviewURL); err != nil {
			// Log but continue
			continue
		}
	}
}

func (a *EmailTokenAPI) baseURL(c *gin.Context) string {
	scheme := "http"
	if c.Request.TLS != nil || c.GetHeader("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return scheme + "://" + c.Request.Host
}
```

- [ ] **Step 2: Add helper methods to UserService**

Add these methods to `internal/server/biz/user.go`:

```go
func (s *UserService) MarkEmailVerified(ctx context.Context, userID int) error {
	_, err := s.entClient.User.UpdateOneID(userID).
		SetEmailVerifiedAt(time.Now()).
		Save(ctx)
	return err
}

func (s *UserService) ActivateUser(ctx context.Context, userID int) error {
	rs, err := s.systemService.RegistrationSettings(ctx)
	if err != nil {
		return err
	}
	_, err = s.entClient.User.UpdateOneID(userID).
		SetStatus(user.StatusActivated).
		SetScopes(rs.DefaultUserScopes).
		Save(ctx)
	return err
}

func (s *UserService) GetByEmail(ctx context.Context, email string) (*ent.User, error) {
	return s.entClient.User.Query().
		Where(user.Email(email)).
		Only(ctx)
}

func (s *UserService) ResetPassword(ctx context.Context, userID int, newPassword string) error {
	hashed, err := bcrypt.GenerateFromPassword([]byte(newPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.entClient.User.UpdateOneID(userID).
		SetPassword(string(hashed)).
		Save(ctx)
	return err
}

func (s *UserService) ListOwners(ctx context.Context) ([]*ent.User, error) {
	return s.entClient.User.Query().
		Where(
			user.Status(user.StatusActivated),
			user.HasRoleWith(role.Name("owner")),
		).
		All(ctx)
}
```

- [ ] **Step 3: Register in FX module**

In `internal/server/api/fx_module.go`, add:

```go
fx.Provide(NewEmailTokenAPI),
```

- [ ] **Step 4: Register routes**

In `internal/server/routes.go`, find the auth route group and add:

```go
authGroup := r.Group("/admin/auth")
// ... existing routes ...
authGroup.GET("/verify-email", emailTokenAPI.VerifyEmail)
authGroup.POST("/resend-verification", emailTokenAPI.ResendVerification)
authGroup.POST("/forgot-password", emailTokenAPI.ForgotPassword)
authGroup.POST("/reset-password", emailTokenAPI.ResetPassword)
```

- [ ] **Step 5: Commit**

```bash
git add internal/server/api/email_token.go internal/server/api/fx_module.go internal/server/routes.go internal/server/biz/user.go
git commit -m "feat: add email token REST handlers for verify-email, forgot-password, reset-password"
```

---

## Task 10: Add GraphQL Types and Resolvers

**Files:**
- Modify: `internal/server/gql/system.graphql`
- Modify: `internal/server/gql/user.graphql`
- Modify: `internal/server/gql/system.resolvers.go`
- Modify: `internal/server/gql/user.resolvers.go`

- [ ] **Step 1: Add types to system.graphql**

Append to the file:

```graphql
type RegistrationSettings {
  allowSignUp: Boolean!
  approvalRequired: Boolean!
  defaultUserScopes: [String!]!
}

input RegistrationSettingsInput {
  allowSignUp: Boolean!
  approvalRequired: Boolean!
  defaultUserScopes: [String!]!
}

type EmailSettings {
  smtpHost: String!
  smtpPort: Int!
  smtpUser: String!
  smtpPassword: String!
  encryption: String!
  fromName: String!
  fromAddress: String!
  connected: Boolean!
}

input EmailSettingsInput {
  smtpHost: String!
  smtpPort: Int!
  smtpUser: String!
  smtpPassword: String!
  encryption: String!
  fromName: String!
  fromAddress: String!
}

type TestEmailResult {
  success: Boolean!
  message: String!
}

extend type Query {
  registrationSettings: RegistrationSettings!
  emailSettings: EmailSettings!
}

extend type Mutation {
  updateRegistrationSettings(input: RegistrationSettingsInput!): RegistrationSettings!
  updateEmailSettings(input: EmailSettingsInput!): EmailSettings!
  testEmailConnection: TestEmailResult!
}
```

- [ ] **Step 2: Add mutations to user.graphql**

Append to the file:

```graphql
extend type Mutation {
  approveUser(id: Int!): User!
  rejectUser(id: Int!): Boolean!
}
```

- [ ] **Step 3: Run GraphQL code generation**

Run: `cd D:\PythonProject\axonhub && go generate ./internal/server/gql`

Expected: Generated resolver stubs appear. Implement them in the next step.

- [ ] **Step 4: Implement system resolvers**

In `internal/server/gql/system.resolvers.go`, add:

```go
func (r *queryResolver) RegistrationSettings(ctx context.Context) (*model.RegistrationSettings, error) {
	rs, err := r.systemService.RegistrationSettings(ctx)
	if err != nil {
		return nil, err
	}
	return &model.RegistrationSettings{
		AllowSignUp:       rs.AllowSignUp,
		ApprovalRequired:  rs.ApprovalRequired,
		DefaultUserScopes: rs.DefaultUserScopes,
	}, nil
}

func (r *queryResolver) EmailSettings(ctx context.Context) (*model.EmailSettings, error) {
	es, err := r.systemService.EmailSettings(ctx)
	if err != nil {
		return nil, err
	}
	return &model.EmailSettings{
		SMTPHost:     es.SMTPHost,
		SMTPPort:     es.SMTPPort,
		SMTPUser:     es.SMTPUser,
		SMTPPassword: "••••••••", // Masked
		Encryption:   es.Encryption,
		FromName:     es.FromName,
		FromAddress:  es.FromAddress,
		Connected:    r.emailService.TestConnection(ctx) == nil,
	}, nil
}

func (r *mutationResolver) UpdateRegistrationSettings(ctx context.Context, input model.RegistrationSettingsInput) (*model.RegistrationSettings, error) {
	rs := &biz.RegistrationSettings{
		AllowSignUp:       input.AllowSignUp,
		ApprovalRequired:  input.ApprovalRequired,
		DefaultUserScopes: input.DefaultUserScopes,
	}
	if err := r.systemService.UpdateRegistrationSettings(ctx, rs); err != nil {
		return nil, err
	}
	return r.queryResolver.RegistrationSettings(ctx)
}

func (r *mutationResolver) UpdateEmailSettings(ctx context.Context, input model.EmailSettingsInput) (*model.EmailSettings, error) {
	es := &biz.EmailSettings{
		SMTPHost:     input.SMTPHost,
		SMTPPort:     input.SMTPPort,
		SMTPUser:     input.SMTPUser,
		SMTPPassword: input.SMTPPassword,
		Encryption:   input.Encryption,
		FromName:     input.FromName,
		FromAddress:  input.FromAddress,
	}
	if err := r.systemService.UpdateEmailSettings(ctx, es); err != nil {
		return nil, err
	}
	return r.queryResolver.EmailSettings(ctx)
}

func (r *mutationResolver) TestEmailConnection(ctx context.Context) (*model.TestEmailResult, error) {
	me, err := r.userService.Me(ctx)
	if err != nil {
		return nil, err
	}
	if err := r.emailService.TestConnection(ctx); err != nil {
		return &model.TestEmailResult{Success: false, Message: err.Error()}, nil
	}
	if err := r.emailService.SendTestEmail(ctx, me.Email); err != nil {
		return &model.TestEmailResult{Success: false, Message: "Connection OK but send failed: " + err.Error()}, nil
	}
	return &model.TestEmailResult{Success: true, Message: "Test email sent successfully"}, nil
}
```

- [ ] **Step 5: Implement user resolvers**

In `internal/server/gql/user.resolvers.go`, add:

```go
func (r *mutationResolver) ApproveUser(ctx context.Context, id int) (*model.User, error) {
	if err := r.userService.ApproveUser(ctx, id); err != nil {
		return nil, err
	}
	u, err := r.userService.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	return r.toModelUser(u), nil
}

func (r *mutationResolver) RejectUser(ctx context.Context, id int) (bool, error) {
	if err := r.userService.RejectUser(ctx, id); err != nil {
		return false, err
	}
	return true, nil
}
```

- [ ] **Step 6: Wire EmailService into the GraphQL server struct**

In the GraphQL server struct (likely in `internal/server/gql/server.go` or the resolver struct), add `emailService *biz.EmailService` field and update the constructor.

- [ ] **Step 7: Commit**

```bash
git add internal/server/gql/
git commit -m "feat: add GraphQL types and resolvers for registration/email settings and user approval"
```

---

## Task 11: Frontend — Sidebar & Dashboard Cleanup

**Files:**
- Modify: `frontend/src/features/dashboard/index.tsx`
- Modify: `frontend/src/features/dashboard/context.tsx`
- Modify: `frontend/src/routes/_authenticated/index.tsx`
- Modify: `frontend/src/sidebar.ts`

- [ ] **Step 1: Remove DashboardModeContext**

In `frontend/src/features/dashboard/context.tsx`, delete the entire file or remove the `DashboardModeContext` and `useDashboardMode` exports.

- [ ] **Step 2: Update Dashboard component**

In `frontend/src/features/dashboard/index.tsx`:
- Remove the `<Tabs>` component that toggles between Project/Personal
- Accept a `mode` prop (`"admin" | "personal"`) instead of reading from context
- Use `mode` to select the correct GraphQL query directly
- Remove all `useDashboardMode()` calls

- [ ] **Step 3: Update personal dashboard route**

In `frontend/src/routes/_authenticated/index.tsx`:
- Remove `DashboardModeContext.Provider` wrapper
- Pass `mode="personal"` to the Dashboard component

- [ ] **Step 4: Update admin dashboard route**

In `frontend/src/routes/_authenticated/admin/index.tsx`:
- Pass `mode="admin"` to the Dashboard component

- [ ] **Step 5: Add Users & Roles to admin sidebar**

In `frontend/src/sidebar.ts`, find the admin sidebar group items array. Add after the last admin item:

```typescript
{ title: 'Users', href: '/admin/users', icon: IconUsers, badge: pendingCount },
{ title: 'Roles', href: '/admin/roles', icon: IconUsersGroup },
```

The `pendingCount` can be fetched from a lightweight field on the `me` query or a dedicated `pendingUsersCount` query.

- [ ] **Step 6: Move user/role routes**

Rename/move:
- `frontend/src/routes/_authenticated/users/` → `frontend/src/routes/_authenticated/admin/users/`
- `frontend/src/routes/_authenticated/roles/` → `frontend/src/routes/_authenticated/admin/roles/`

Update the route path definitions inside each to use `/admin/users` and `/admin/roles`.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/features/dashboard/ frontend/src/routes/ frontend/src/sidebar.ts
git commit -m "feat: remove dashboard toggle, add Users/Roles to admin sidebar"
```

---

## Task 12: Frontend — Registration Settings Tab

**Files:**
- Create: `frontend/src/features/system/components/registration-settings.tsx`
- Modify: `frontend/src/features/system/components/tabs.tsx`

- [ ] **Step 1: Create RegistrationSettings component**

Create `frontend/src/features/system/components/registration-settings.tsx` following the existing pattern of other settings tab components (e.g., `general-settings.tsx`). The component should:

- Fetch `registrationSettings` via GraphQL query
- Display a toggle for "Allow Self-Registration"
- When enabled, show a mode selector: "Auto-approve" / "Require admin approval"
- Show default scopes multi-select (reuse existing scope selection patterns from the roles feature)
- Save via `updateRegistrationSettings` mutation

- [ ] **Step 2: Add tab to System Settings**

In `frontend/src/features/system/components/tabs.tsx`, add the "Registration" tab entry pointing to the new component.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/system/components/registration-settings.tsx frontend/src/features/system/components/tabs.tsx
git commit -m "feat: add Registration Settings tab to System Settings"
```

---

## Task 13: Frontend — Email Settings Tab

**Files:**
- Create: `frontend/src/features/system/components/email-settings.tsx`
- Modify: `frontend/src/features/system/components/tabs.tsx`

- [ ] **Step 1: Create EmailSettings component**

Create `frontend/src/features/system/components/email-settings.tsx` following the existing pattern. The component should:

- Fetch `emailSettings` via GraphQL query
- Display form fields: SMTP Host, Port, Username, Password, Encryption (radio: SSL/TLS, STARTTLS, None), From Name, From Address
- Show connection status indicator (green/red dot based on `connected` field)
- "Send Test Email" button — calls `testEmailConnection` mutation, shows success/error toast
- Save via `updateEmailSettings` mutation
- Password field is pre-filled with masked value; only sent if user changes it

- [ ] **Step 2: Add tab to System Settings**

In `frontend/src/features/system/components/tabs.tsx`, add the "Email" tab entry.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/system/components/email-settings.tsx frontend/src/features/system/components/tabs.tsx
git commit -m "feat: add Email Settings tab to System Settings"
```

---

## Task 14: Frontend — Update Sign-Up Page

**Files:**
- Modify: `frontend/src/features/auth/sign-up/` (all relevant files)

- [ ] **Step 1: Remove auto-login after signup**

Find the signup mutation's `onSuccess` handler. Currently it likely calls a login function or stores a session token. Change it to:

1. Show a success state: "Registration successful! Please check your email to verify your account."
2. Display the email address the user signed up with
3. Show a "Resend verification email" link
4. Show a "Back to Sign In" link

- [ ] **Step 2: Add resend verification handler**

Add a `resendVerification` function that calls `POST /admin/auth/resend-verification` with `{ email }`. Rate-limited on the backend. Show a toast on success.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/auth/sign-up/
git commit -m "feat: signup shows email verification prompt instead of auto-login"
```

---

## Task 15: Frontend — Create Verify Email Page

**Files:**
- Create: `frontend/src/routes/(auth)/verify-email.tsx`

- [ ] **Step 1: Create the verify-email route page**

This page is reached via redirect from the backend `GET /admin/auth/verify-email` endpoint. It reads URL params:

- `verified=1` → Show "Email verified!" success message with "Sign In" button
- `verified=1&pending=1` → Show "Awaiting admin approval" message
- `verified=0` → Show "Verification link invalid or expired" error message

```tsx
import { createFileRoute } from '@tanstack/react-router';
import { useSearch } from '@tanstack/react-router';

export const Route = createFileRoute('/(auth)/verify-email')({
  component: VerifyEmailPage,
});

function VerifyEmailPage() {
  const search = useSearch({ strict: false }) as { verified?: string; pending?: string };

  if (search.verified === '1' && search.pending === '1') {
    // Awaiting approval
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center max-w-md">
          {/* Clock icon */}
          <h2>Awaiting Approval</h2>
          <p>Your email has been verified. An administrator needs to approve your account before you can sign in.</p>
          <p>You'll receive an email once your account is reviewed.</p>
          <a href="/sign-in">Back to Sign In</a>
        </div>
      </div>
    );
  }

  if (search.verified === '1') {
    // Verified successfully
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center max-w-md">
          {/* Check icon */}
          <h2>Email Verified!</h2>
          <p>Your email has been verified successfully. You can now sign in to your account.</p>
          <a href="/sign-in">Sign In</a>
        </div>
      </div>
    );
  }

  // Invalid/expired
  return (
    <div className="flex items-center justify-center min-h-screen">
      <div className="text-center max-w-md">
        {/* Error icon */}
        <h2>Verification Failed</h2>
        <p>The verification link is invalid or has expired.</p>
        <a href="/sign-up">Sign Up Again</a>
      </div>
    </div>
  );
}
```

Style with Tailwind CSS following the existing auth page patterns.

- [ ] **Step 2: Commit**

```bash
git add frontend/src/routes/(auth)/verify-email.tsx
git commit -m "feat: add verify-email result page"
```

---

## Task 16: Frontend — Replace Forgot Password Stub

**Files:**
- Modify: `frontend/src/features/auth/forgot-password/` (all relevant files)
- Modify: `frontend/src/routes/(auth)/forgot-password.tsx`

- [ ] **Step 1: Replace the stub with real implementation**

Update the forgot-password feature to:

1. Show an email input form
2. On submit, call `POST /admin/auth/forgot-password` with `{ email }`
3. On success, show "If the email exists, a reset link has been sent." message
4. Show "Back to Sign In" link

- [ ] **Step 2: Commit**

```bash
git add frontend/src/features/auth/forgot-password/ frontend/src/routes/(auth)/forgot-password.tsx
git commit -m "feat: replace forgot-password stub with real email-based flow"
```

---

## Task 17: Frontend — Create Reset Password Page

**Files:**
- Create: `frontend/src/routes/(auth)/reset-password.tsx`

- [ ] **Step 1: Create the reset-password route page**

This page is reached via the email reset link. It reads the `token` from the URL.

```tsx
import { createFileRoute } from '@tanstack/react-router';
import { useSearch } from '@tanstack/react-router';
import { useState } from 'react';

export const Route = createFileRoute('/(auth)/reset-password')({
  component: ResetPasswordPage,
});

function ResetPasswordPage() {
  const search = useSearch({ strict: false }) as { token?: string };
  const [password, setPassword] = useState('');
  const [confirmPassword, setConfirmPassword] = useState('');
  const [success, setSuccess] = useState(false);
  const [error, setError] = useState('');

  if (!search.token) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center max-w-md">
          <h2>Invalid Link</h2>
          <p>This password reset link is invalid.</p>
          <a href="/forgot-password">Request a new link</a>
        </div>
      </div>
    );
  }

  if (success) {
    return (
      <div className="flex items-center justify-center min-h-screen">
        <div className="text-center max-w-md">
          <h2>Password Reset</h2>
          <p>Your password has been reset successfully.</p>
          <a href="/sign-in">Sign In</a>
        </div>
      </div>
    );
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    if (password !== confirmPassword) {
      setError('Passwords do not match');
      return;
    }
    if (password.length < 8) {
      setError('Password must be at least 8 characters');
      return;
    }
    try {
      const res = await fetch('/admin/auth/reset-password', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ token: search.token, password }),
      });
      if (!res.ok) {
        const data = await res.json();
        setError(data.message || 'Failed to reset password');
        return;
      }
      setSuccess(true);
    } catch {
      setError('Network error');
    }
  };

  return (
    <div className="flex items-center justify-center min-h-screen">
      <div className="max-w-md w-full">
        <h2>Reset Password</h2>
        <p>Enter your new password</p>
        <form onSubmit={handleSubmit}>
          <input type="password" placeholder="New Password" value={password} onChange={e => setPassword(e.target.value)} required minLength={8} />
          <input type="password" placeholder="Confirm New Password" value={confirmPassword} onChange={e => setConfirmPassword(e.target.value)} required minLength={8} />
          {error && <p className="text-red-500">{error}</p>}
          <button type="submit">Reset Password</button>
        </form>
      </div>
    </div>
  );
}
```

Style with Tailwind CSS following the existing auth page patterns.

- [ ] **Step 2: Commit**

```bash
git add frontend/src/routes/(auth)/reset-password.tsx
git commit -m "feat: add reset-password page with token-validated form"
```

---

## Task 18: Frontend — Update Sign-In Page

**Files:**
- Modify: `frontend/src/features/auth/sign-in/` (main component)

- [ ] **Step 1: Handle verified/pending URL params**

In the sign-in component, read URL search params on mount:

- `verified=1` → Show success toast: "Email verified! Please sign in."
- `verified=1&pending=1` → Show info message: "Your email has been verified. An administrator needs to approve your account."
- `verified=0` → Show error toast: "Verification link is invalid or expired."

- [ ] **Step 2: Ensure "Forgot password?" link works**

Verify the "Forgot password?" link points to `/forgot-password` (it should already, since we replaced the stub in Task 16).

- [ ] **Step 3: Commit**

```bash
git add frontend/src/features/auth/sign-in/
git commit -m "feat: sign-in page handles email verification result params"
```

---

## Task 19: Frontend — Users Page Approval UI

**Files:**
- Modify: `frontend/src/routes/_authenticated/admin/users/index.tsx` (or the users feature component)

- [ ] **Step 1: Add pending user highlighting and approval actions**

In the Users page component:

1. Add a status filter (All / Pending / Active / Deactivated)
2. Highlight pending users with a yellow row background
3. For pending users, show "Approve" (green) and "Reject" (red) action buttons
4. Approve calls `approveUser` mutation
5. Reject calls `rejectUser` mutation (with a confirmation dialog)
6. On success, invalidate the users query to refresh the list

- [ ] **Step 2: Add pending count to the page header**

Show "N pending approval" badge in the page header when pending users exist.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes/_authenticated/admin/users/
git commit -m "feat: add user approval/rejection UI to admin users page"
```

---

## Task 20: Frontend — i18n Keys

**Files:**
- Modify: `frontend/src/locales/en.json`
- Modify: `frontend/src/locales/zh.json`

- [ ] **Step 1: Add English i18n keys**

Add keys for all new UI text:

```json
{
  "registration": {
    "title": "Registration",
    "allowSignUp": "Allow Self-Registration",
    "allowSignUpDesc": "Enable new users to create their own accounts",
    "mode": "Registration Mode",
    "autoApprove": "Auto-approve",
    "requireApproval": "Require admin approval",
    "autoApproveDesc": "Users can access the system immediately after email verification.",
    "requireApprovalDesc": "Admin must approve after email verification.",
    "defaultScopes": "Default Scopes for New Users",
    "defaultScopesDesc": "These permissions are automatically assigned to self-registered users."
  },
  "email": {
    "title": "Email Service (SMTP)",
    "smtpHost": "SMTP Host",
    "smtpPort": "SMTP Port",
    "smtpUser": "Username",
    "smtpPassword": "Password",
    "encryption": "Encryption",
    "fromName": "From Name",
    "fromAddress": "From Address",
    "connected": "Connected",
    "disconnected": "Not connected",
    "sendTestEmail": "Send Test Email",
    "testSuccess": "Test email sent successfully",
    "testFailed": "Test failed"
  },
  "verifyEmail": {
    "success": "Email Verified!",
    "successDesc": "Your email has been verified successfully. You can now sign in to your account.",
    "pending": "Awaiting Approval",
    "pendingDesc": "Your email has been verified. An administrator needs to approve your account before you can sign in.",
    "pendingDesc2": "You'll receive an email once your account is reviewed.",
    "failed": "Verification Failed",
    "failedDesc": "The verification link is invalid or has expired.",
    "checkEmail": "Check Your Email",
    "checkEmailDesc": "We've sent a verification link to",
    "linkExpires": "Click the link in the email to verify your account. The link expires in 24 hours.",
    "resend": "Resend verification email",
    "notReceived": "Didn't receive the email?"
  },
  "forgotPassword": {
    "title": "Forgot Password?",
    "desc": "Enter your email to receive a reset link",
    "sent": "If the email exists, a reset link has been sent."
  },
  "resetPassword": {
    "title": "Reset Password",
    "desc": "Enter your new password",
    "success": "Your password has been reset successfully.",
    "invalidLink": "This password reset link is invalid.",
    "requestNew": "Request a new link"
  },
  "users": {
    "pendingApproval": "pending approval",
    "approve": "Approve",
    "reject": "Reject",
    "rejectConfirm": "Are you sure you want to reject this user? This action cannot be undone.",
    "status": {
      "pending": "Pending",
      "activated": "Active",
      "deactivated": "Deactivated"
    }
  }
}
```

- [ ] **Step 2: Add Chinese i18n keys**

Add corresponding Chinese translations.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/locales/en.json frontend/src/locales/zh.json
git commit -m "feat: add i18n keys for registration, email, and auth pages"
```

---

## Task 21: GraphQL Codegen + Final Frontend Build

**Files:**
- Regenerate: `frontend/src/gql/` (via codegen)

- [ ] **Step 1: Run GraphQL codegen**

Run: `cd frontend && npm run codegen` (or the project's codegen command)

Expected: New types for `RegistrationSettings`, `EmailSettings`, `approveUser`, `rejectUser`, etc. appear in the generated files.

- [ ] **Step 2: Verify frontend builds**

Run: `cd frontend && npm run build`

Expected: No TypeScript errors.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/gql/
git commit -m "feat: regenerate GraphQL types for registration and email settings"
```

---

## Task 22: Backend Build Verification

- [ ] **Step 1: Run Go build**

Run: `cd D:\PythonProject\axonhub && go build ./...`

Expected: No compilation errors.

- [ ] **Step 2: Run Go vet**

Run: `cd D:\PythonProject\axonhub && go vet ./...`

Expected: No issues.

- [ ] **Step 3: Run existing tests**

Run: `cd D:\PythonProject\axonhub && go test ./internal/...`

Expected: All existing tests pass.

---

## Task 23: End-to-End Verification

- [ ] **Step 1: Sidebar** — Open app, verify no Project/Personal toggle on dashboard, verify Users/Roles appear in admin sidebar
- [ ] **Step 2: Registration settings** — Go to /admin/system → Registration tab, toggle settings, verify they persist after page reload
- [ ] **Step 3: Email settings** — Go to /admin/system → Email tab, configure SMTP, click "Send Test Email", verify delivery
- [ ] **Step 4: Registration (auto-approve)** — Sign up as new user, verify email received, click link, verify redirected to sign-in with success message, verify can sign in
- [ ] **Step 5: Registration (approval-required)** — Sign up as new user, verify email, click link, verify "awaiting approval" message, verify admin gets notification, admin approves, verify user gets approval email, verify user can sign in
- [ ] **Step 6: Rejection** — Admin rejects a pending user, verify user gets rejection email, verify user record is deleted
- [ ] **Step 7: Forgot password** — Click "Forgot password?", enter email, verify reset email received, click link, set new password, verify can sign in with new password
- [ ] **Step 8: Token expiry** — Manually set a token expired in DB, verify verification/reset links fail gracefully
- [ ] **Step 9: Resend verification** — After sign-up, click "Resend verification email", verify new email sent
- [ ] **Step 10: Pending badge** — When pending users exist, verify badge count shows on Users sidebar item
