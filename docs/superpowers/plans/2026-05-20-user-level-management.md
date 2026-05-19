# User-Level Management Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Transform AxonHub from project-based isolation to user-level management where each user owns and manages their own channels, models, API keys, prompts, and requests.

**Architecture:** Keep the Project table as an internal implementation detail — auto-create one private project per user. Add ownership and visibility fields to Channel/Model. Introduce a PublishRequest entity for admin-reviewed publishing. Add user self-registration with configurable approval.

**Tech Stack:** Go 1.26+, Ent ORM, gqlgen, Gin, React 19, TypeScript, TanStack Query/Router, Zustand, Tailwind CSS

---

## File Structure

### New Files

| File | Responsibility |
|------|---------------|
| `internal/ent/schema/publish_request.go` | PublishRequest entity schema |
| `internal/server/biz/signup.go` | User self-registration logic |
| `internal/server/biz/publish_request.go` | Publish request CRUD + review logic |
| `internal/server/gql/publish_request.graphql` | Publish request GraphQL schema |
| `internal/server/gql/publish_request.resolvers.go` | Publish request resolvers |
| `internal/server/api/signup.go` | REST endpoint for sign-up |
| `internal/scopes/rule_channel_visibility.go` | Channel/Model visibility-based privacy rules |
| `frontend/src/routes/_authenticated/sign-up/index.tsx` | Registration page |
| `frontend/src/features/publish-requests/index.tsx` | Admin review queue page |
| `frontend/src/features/publish-requests/data/publish-requests.ts` | Publish request GraphQL hooks |
| `frontend/src/features/publish-requests/components/publish-request-card.tsx` | Review card component |
| `frontend/src/features/my-dashboard/index.tsx` | User-scoped dashboard page |

### Modified Files

| File | Change |
|------|--------|
| `internal/ent/schema/channel.go` | Add `owner_id`, `visibility`, `shared_with` fields + edge to User |
| `internal/ent/schema/model.go` | Add `owner_id`, `visibility`, `shared_with` fields + edge to User |
| `internal/ent/schema/user.go` | Add `private_project_id` field |
| `internal/scopes/scopes.go` | Add 3 new scope constants + configs |
| `internal/scopes/rule_user_scope.go` | Update Channel/Model read/write rules to use visibility |
| `internal/server/biz/user.go` | Auto-create private project on CreateUser; add SignUp method |
| `internal/server/biz/project.go` | Add CreatePrivateProject helper |
| `internal/server/biz/oidc.go` | Respect `allow_sign_up` setting in JIT provisioning |
| `internal/server/biz/channel.go` | Set `owner_id` on create; visibility-aware list queries |
| `internal/server/biz/model.go` | Set `owner_id` on create; visibility-aware list queries |
| `internal/server/gql/axonhub.graphql` | Add new mutations, queries, types, enums |
| `internal/server/gql/dashboard.resolvers.go` | Add user-scoped dashboard resolver |
| `internal/server/gql/channel.resolvers.go` | Add share/unshare/requestPublish resolvers |
| `internal/server/routes.go` | Add sign-up REST route |
| `internal/ent/migrate/datamigrate/migrator.go` | Add migration for existing data |
| `frontend/src/sidebar.ts` | Restructure navigation groups |
| `frontend/src/stores/projectStore.ts` | Replace with auto-resolved privateProjectId |
| `frontend/src/components/layout/project-switcher.tsx` | Remove |
| `frontend/src/components/project-guard.tsx` | Remove |
| `frontend/src/features/channels/data/channels.ts` | Add share/unshare/publish hooks |
| `frontend/src/features/channels/components/channel-share-dialog.tsx` | New sharing UI |
| `frontend/src/features/models/data/models.ts` | Add share/unshare/publish hooks |
| `frontend/src/features/models/components/model-share-dialog.tsx` | New sharing UI |
| `frontend/src/features/dashboard/index.tsx` | Split into my-dashboard + admin-dashboard |
| `frontend/src/locales/en.json` | New i18n keys |
| `frontend/src/locales/zh.json` | New i18n keys |

---

### Task 1: Add new scope constants

**Files:**
- Modify: `internal/scopes/scopes.go`

- [ ] **Step 1: Add three new scope constants**

Add after the existing `ScopeWritePrompts` block:

```go
// ScopeManageOwnChannels manage own channels (where owner_id = current user).
ScopeManageOwnChannels ScopeSlug = "manage_own_channels"
// ScopeManageOwnModels manage own models (where owner_id = current user).
ScopeManageOwnModels ScopeSlug = "manage_own_models"
// ScopeReviewPublishRequests review publish requests (admin).
ScopeReviewPublishRequests ScopeSlug = "review_publish_requests"
```

- [ ] **Step 2: Add scope configs**

Add three entries to the `scopeConfigs` slice:

```go
{
    Slug:        ScopeManageOwnChannels,
    Description: "Manage own channels",
    Levels:      []ScopeLevel{ScopeLevelSystem},
},
{
    Slug:        ScopeManageOwnModels,
    Description: "Manage own models",
    Levels:      []ScopeLevel{ScopeLevelSystem},
},
{
    Slug:        ScopeReviewPublishRequests,
    Description: "Review publish requests",
    Levels:      []ScopeLevel{ScopeLevelSystem},
},
```

- [ ] **Step 3: Commit**

```bash
git add internal/scopes/scopes.go
git commit -m "feat(scopes): add manage_own_channels, manage_own_models, review_publish_requests"
```

---

### Task 2: Add Channel/Model ownership and visibility fields

**Files:**
- Modify: `internal/ent/schema/channel.go`
- Modify: `internal/ent/schema/model.go`

- [ ] **Step 1: Add fields to Channel schema**

In `channel.go`, add these fields after the existing `endpoints` field:

```go
field.Int("owner_id").Optional().Immutable().Annotations(entgql.Skip(Create, Update)),
field.Enum("visibility").Values("private", "shared", "published").Default("private").Annotations(entgql.OrderField("VISIBILITY")),
field.JSON("shared_with", []int{}).Optional().Annotations(entgql.Skip(Create, Update)),
```

Add an edge to User:

```go
edge.From("owner", User.Type).Ref("owned_channels").Unique().Field("owner_id").Annotations(entgql.Skip(Create, Update)),
```

Add an index:

```go
index.Fields("owner_id", "deleted_at").StorageKey("channels_by_owner_id"),
```

- [ ] **Step 2: Add same fields to Model schema**

In `model.go`, add after the `remark` field:

```go
field.Int("owner_id").Optional().Immutable().Annotations(entgql.Skip(Create, Update)),
field.Enum("visibility").Values("private", "shared", "published").Default("private").Annotations(entgql.OrderField("VISIBILITY")),
field.JSON("shared_with", []int{}).Optional().Annotations(entgql.Skip(Create, Update)),
```

Add edge:

```go
edge.From("owner", User.Type).Ref("owned_models").Unique().Field("owner_id").Annotations(entgql.Skip(Create, Update)),
```

Add index:

```go
index.Fields("owner_id", "deleted_at").StorageKey("models_by_owner_id"),
```

- [ ] **Step 3: Add inverse edges to User schema**

In `user.go`, add edges:

```go
edge.To("owned_channels", Channel.Type).Annotations(entgql.Skip(Create, Update)),
edge.To("owned_models", Model.Type).Annotations(entgql.Skip(Create, Update)),
```

- [ ] **Step 4: Regenerate ent code**

```bash
cd D:/PythonProject/axonhub && go generate ./internal/ent
```

- [ ] **Step 5: Commit**

```bash
git add internal/ent/schema/channel.go internal/ent/schema/model.go internal/ent/schema/user.go internal/ent/
git commit -m "feat(schema): add owner_id, visibility, shared_with to Channel and Model"
```

---

### Task 3: Add private_project_id to User schema

**Files:**
- Modify: `internal/ent/schema/user.go`

- [ ] **Step 1: Add field**

In `user.go`, add after the `scopes` field:

```go
field.Int("private_project_id").Optional().Nillable().Immutable().Annotations(entgql.Skip(Create, Update)),
```

Add edge:

```go
edge.To("private_project", Project.Type).Unique().Field("private_project_id").Annotations(entgql.Skip(Create, Update)),
```

- [ ] **Step 2: Regenerate ent code**

```bash
cd D:/PythonProject/axonhub && go generate ./internal/ent
```

- [ ] **Step 3: Commit**

```bash
git add internal/ent/schema/user.go internal/ent/
git commit -m "feat(schema): add private_project_id to User"
```

---

### Task 4: Create PublishRequest schema

**Files:**
- Create: `internal/ent/schema/publish_request.go`

- [ ] **Step 1: Write the schema**

```go
package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"github.com/ldm2060/axonhub/internal/ent/schema/schematype"
	"github.com/ldm2060/axonhub/internal/pkg/entgql"
)

type PublishRequest struct {
	ent.Schema
}

func (PublishRequest) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (PublishRequest) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (PublishRequest) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("resource_type").Values("channel", "model").Immutable(),
		field.Int("resource_id").Immutable(),
		field.Int("requester_id").Immutable().Annotations(entgql.Skip(Create, Update)),
		field.Enum("status").Values("pending", "approved", "rejected").Default("pending").Annotations(entgql.Skip(Create)),
		field.Int("reviewer_id").Optional().Nillable().Immutable().Annotations(entgql.Skip(Create, Update)),
		field.String("review_comment").Optional().Nillable(),
		field.String("request_comment").Optional().Nillable(),
	}
}

func (PublishRequest) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("requester", User.Type).Ref("publish_requests").Unique().Required().Field("requester_id").Annotations(entgql.Skip(Create, Update)),
		edge.From("reviewer", User.Type).Ref("reviewed_requests").Unique().Field("reviewer_id").Annotations(entgql.Skip(Create, Update)),
	}
}

func (PublishRequest) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "deleted_at").StorageKey("publish_requests_by_status"),
		index.Fields("requester_id", "deleted_at").StorageKey("publish_requests_by_requester"),
	}
}

func (PublishRequest) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.OwnerRule(),
			scopes.UserReadScopeRule(scopes.ScopeReviewPublishRequests),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeReviewPublishRequests),
		},
	}
}
```

- [ ] **Step 2: Add inverse edges to User schema**

In `user.go`, add:

```go
edge.To("publish_requests", PublishRequest.Type).Annotations(entgql.Skip(Create, Update)),
edge.To("reviewed_requests", PublishRequest.Type).Annotations(entgql.Skip(Create, Update)),
```

- [ ] **Step 3: Regenerate ent code**

```bash
cd D:/PythonProject/axonhub && go generate ./internal/ent
```

- [ ] **Step 4: Commit**

```bash
git add internal/ent/schema/publish_request.go internal/ent/schema/user.go internal/ent/
git commit -m "feat(schema): add PublishRequest entity"
```

---

### Task 5: Implement Channel/Model visibility privacy rules

**Files:**
- Create: `internal/scopes/rule_channel_visibility.go`
- Modify: `internal/ent/schema/channel.go` (Policy)
- Modify: `internal/ent/schema/model.go` (Policy)

- [ ] **Step 1: Write visibility-based query rule**

Create `internal/scopes/rule_channel_visibility.go`:

```go
package scopes

import (
	"context"
	"encoding/json"

	"entgo.io/ent/dialect/sql"
	"entgo.io/ent/entql"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/privacy"
)

// ChannelVisibilityFilter is implemented by Channel and Model queries.
type ChannelVisibilityFilter interface {
	WhereOwnerID(entql.IntP)
	WhereVisibility(entql.StringP)
	WhereOr(...func(entql.Querier))
}

// ChannelVisibilityQueryRule allows reading channels/models based on visibility:
// 1. published → all users
// 2. owner_id = current user → own resources
// 3. shared_with contains current user ID → shared resources
func ChannelVisibilityQueryRule(readScope ScopeSlug) privacy.QueryRule {
	return privacy.FilterFunc(channelVisibilityFilter(readScope))
}

func channelVisibilityFilter(readScope ScopeSlug) func(ctx context.Context, q privacy.Filter) error {
	return func(ctx context.Context, q privacy.Filter) error {
		user, err := getUserFromContext(ctx)
		if err != nil {
			return err
		}

		// Admin with global read scope sees everything
		if userHasSystemScope(user, readScope) {
			return privacy.Allow
		}

		switch q := q.(type) {
		case ChannelVisibilityFilter:
			// Filter: visibility = "published" OR owner_id = current user
			q.WhereOr(
				func(s *entql.Selector) {
					s.Where(entql.Or(
						entql.EQ("visibility", "published"),
						entql.EQ("owner_id", user.ID),
					))
				},
			)
			// Also include shared_with containing user ID (checked in application code
			// since JSON array containment is DB-specific)
			return privacy.Allowf("User %d can query visible channels/models", user.ID)
		default:
			return privacy.Skipf("Not a channel visibility query")
		}
	}
}

// ChannelManageOwnMutationRule allows users with manage_own_channels to create/update/delete
// channels where they are the owner.
func ChannelManageOwnMutationRule(manageScope ScopeSlug) privacy.MutationRule {
	return privacy.MutationRuleFunc(func(ctx context.Context, m ent.Mutation) error {
		user, err := getUserFromContext(ctx)
		if err != nil {
			return err
		}

		if !userHasSystemScope(user, manageScope) {
			return privacy.Skipf("User %d does not have scope %s", user.ID, manageScope)
		}

		type ownerMutation interface {
			ent.Mutation
			OwnerID() (r int, exists bool)
			WhereP(ps ...func(*sql.Selector))
		}

		switch mutation := m.(type) {
		case ownerMutation:
			switch mutation.Op() {
			case ent.OpCreate:
				ownerId, ok := mutation.OwnerID()
				if !ok || ownerId != user.ID {
					return privacy.Skipf("User %d can only create own resources", user.ID)
				}
				return privacy.Allowf("User %d can create own resources", user.ID)
			case ent.OpUpdateOne, ent.OpDeleteOne:
				mutation.WhereP(func(s *sql.Selector) {
					s.Where(sql.EQ("owner_id", user.ID))
				})
				return privacy.Allowf("User %d can modify own resources", user.ID)
			case ent.OpUpdate, ent.OpDelete:
				mutation.WhereP(func(s *sql.Selector) {
					s.Where(sql.EQ("owner_id", user.ID))
				})
				return privacy.Allowf("User %d can modify own resources", user.ID)
			default:
				return privacy.Denyf("Unsupported operation %s", mutation.Op())
			}
		default:
			return privacy.Skipf("Not an owner mutation")
		}
	})
}

// isSharedWithUser checks if a user ID is in the shared_with JSON array.
// Used in application-level filtering after DB query.
func isSharedWithUser(sharedWithJSON []byte, userID int) bool {
	if len(sharedWithJSON) == 0 {
		return false
	}
	var ids []int
	if err := json.Unmarshal(sharedWithJSON, &ids); err != nil {
		return false
	}
	for _, id := range ids {
		if id == userID {
			return true
		}
	}
	return false
}
```

- [ ] **Step 2: Update Channel Policy**

Replace Channel's Policy in `channel.go`:

```go
func (Channel) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.APIKeyScopeQueryRule(scopes.ScopeReadChannels),
			scopes.OwnerRule(),
			scopes.ChannelVisibilityQueryRule(scopes.ScopeReadChannels),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWriteChannels),
			scopes.ChannelManageOwnMutationRule(scopes.ScopeManageOwnChannels),
		},
	}
}
```

- [ ] **Step 3: Update Model Policy**

Replace Model's Policy in `model.go`:

```go
func (Model) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.APIKeyScopeQueryRule(scopes.ScopeReadChannels),
			scopes.OwnerRule(),
			scopes.ChannelVisibilityQueryRule(scopes.ScopeReadChannels),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWriteChannels),
			scopes.ChannelManageOwnMutationRule(scopes.ScopeManageOwnModels),
		},
	}
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/scopes/rule_channel_visibility.go internal/ent/schema/channel.go internal/ent/schema/model.go
git commit -m "feat(scopes): add visibility-based privacy rules for Channel and Model"
```

---

### Task 6: Implement private project auto-creation

**Files:**
- Modify: `internal/server/biz/project.go`
- Modify: `internal/server/biz/user.go`

- [ ] **Step 1: Add CreatePrivateProject to project.go**

Add a new method to `ProjectService`:

```go
func (s *ProjectService) CreatePrivateProject(ctx context.Context, userID int) (int, error) {
	projectName := fmt.Sprintf("__user_%d__", userID)
	client := s.entFromContext(ctx)

	proj, err := client.Project.Create().
		SetName(projectName).
		SetDescription("Private project for user " + fmt.Sprint(userID)).
		Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to create private project: %w", err)
	}

	// Assign user as project owner
	_, err = client.UserProject.Create().
		SetUserID(userID).
		SetProjectID(proj.ID).
		SetIsOwner(true).
		SetScopes([]string{}).
		Save(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to assign project owner: %w", err)
	}

	// Create default project roles (Admin, Developer, Viewer)
	s.createDefaultProjectRoles(ctx, proj.ID)

	return proj.ID, nil
}

func (s *ProjectService) createDefaultProjectRoles(ctx context.Context, projectID int) {
	client := s.entFromContext(ctx)
	roles := []struct {
		name   string
		scopes []string
	}{
		{"Admin", []string{string(scopes.ScopeReadUsers), string(scopes.ScopeWriteUsers), string(scopes.ScopeReadRoles), string(scopes.ScopeWriteRoles), string(scopes.ScopeReadAPIKeys), string(scopes.ScopeWriteAPIKeys), string(scopes.ScopeReadRequests), string(scopes.ScopeWriteRequests)}},
		{"Developer", []string{string(scopes.ScopeReadUsers), string(scopes.ScopeReadAPIKeys), string(scopes.ScopeWriteAPIKeys), string(scopes.ScopeReadRequests)}},
		{"Viewer", []string{string(scopes.ScopeReadUsers), string(scopes.ScopeReadRequests)}},
	}
	for _, r := range roles {
		client.Role.Create().
			SetName(r.name).
			SetLevel(role.LevelProject).
			SetProjectID(projectID).
			SetScopes(r.scopes).
			Save(ctx)
	}
}
```

- [ ] **Step 2: Update CreateUser to auto-create private project**

In `user.go`, modify `CreateUser` to call `CreatePrivateProject` after saving the user, then update `private_project_id`:

```go
func (s *UserService) CreateUser(ctx context.Context, input ent.CreateUserInput) (*ent.User, error) {
	// ... existing password hashing and creation logic ...

	user, err := mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Auto-create private project
	projectID, err := s.projectService.CreatePrivateProject(ctx, user.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to create private project: %w", err)
	}

	// Update user with private_project_id
	user, err = s.entFromContext(ctx).User.UpdateOneID(user.ID).
		SetPrivateProjectID(projectID).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to set private project: %w", err)
	}

	return user, nil
}
```

- [ ] **Step 3: Add EnsurePrivateProject helper for existing users**

```go
func (s *UserService) EnsurePrivateProject(ctx context.Context, user *ent.User) (int, error) {
	if user.PrivateProjectID != nil {
		return *user.PrivateProjectID, nil
	}

	projectID, err := s.projectService.CreatePrivateProject(ctx, user.ID)
	if err != nil {
		return 0, err
	}

	_, err = s.entFromContext(ctx).User.UpdateOneID(user.ID).
		SetPrivateProjectID(projectID).
		Save(ctx)
	if err != nil {
		return 0, err
	}

	return projectID, nil
}
```

- [ ] **Step 4: Commit**

```bash
git add internal/server/biz/project.go internal/server/biz/user.go
git commit -m "feat(biz): auto-create private project on user creation"
```

---

### Task 7: Implement user self-registration (sign-up)

**Files:**
- Create: `internal/server/biz/signup.go`
- Create: `internal/server/api/signup.go`
- Modify: `internal/server/routes.go`

- [ ] **Step 1: Write signup biz logic**

Create `internal/server/biz/signup.go`:

```go
package biz

import (
	"context"
	"fmt"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/user"
)

var (
	DefaultUserScopes = []string{
		string(scopes.ScopeManageOwnChannels),
		string(scopes.ScopeManageOwnModels),
		string(scopes.ScopeReadChannels),
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
		string(scopes.ScopeReadRequests),
		string(scopes.ScopeWriteRequests),
		string(scopes.ScopeReadPrompts),
		string(scopes.ScopeWritePrompts),
	}
)

type SignUpInput struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type SignUpService struct {
	userService    *UserService
	authService    *AuthService
	systemService  *SystemService
}

func NewSignUpService(userService *UserService, authService *AuthService, systemService *SystemService) *SignUpService {
	return &SignUpService{
		userService:   userService,
		authService:   authService,
		systemService: systemService,
	}
}

func (s *SignUpService) SignUp(ctx context.Context, input SignUpInput) (*ent.User, string, error) {
	// Check if sign-up is allowed
	allowSignUp, _ := s.systemService.GetBoolSetting(ctx, "allow_sign_up")
	if !allowSignUp {
		return nil, "", fmt.Errorf("sign-up is not allowed")
	}

	// Check if email already exists
	exists, err := s.userService.entFromContext(ctx).User.Query().
		Where(user.EmailEQ(input.Email)).Exist(ctx)
	if err != nil {
		return nil, "", fmt.Errorf("failed to check existing user: %w", err)
	}
	if exists {
		return nil, "", fmt.Errorf("email already registered")
	}

	// Determine initial status
	approvalRequired, _ := s.systemService.GetBoolSetting(ctx, "sign_up_approval_required")
	status := user.StatusActivated
	if approvalRequired {
		status = user.StatusDeactivated
	}

	// Create user
	userInput := ent.CreateUserInput{
		Email:     input.Email,
		Password:  input.Password,
		FirstName: &input.FirstName,
		LastName:  &input.LastName,
		Status:    &status,
		Scopes:    DefaultUserScopes,
	}

	newUser, err := s.userService.CreateUser(ctx, userInput)
	if err != nil {
		return nil, "", fmt.Errorf("failed to create user: %w", err)
	}

	// If no approval required, generate JWT
	var token string
	if !approvalRequired {
		token, err = s.authService.GenerateJWTToken(ctx, newUser)
		if err != nil {
			return nil, "", fmt.Errorf("failed to generate token: %w", err)
		}
	}

	return newUser, token, nil
}
```

- [ ] **Step 2: Write signup REST endpoint**

Create `internal/server/api/signup.go`:

```go
package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/internal/server/biz"
)

type SignUpHandler struct {
	signUpService *biz.SignUpService
}

func NewSignUpHandler(signUpService *biz.SignUpService) *SignUpHandler {
	return &SignUpHandler{signUpService: signUpService}
}

type SignUpRequest struct {
	Email     string `json:"email" binding:"required,email"`
	Password  string `json:"password" binding:"required,min=8"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type SignUpResponse struct {
	User  *objects.UserInfo `json:"user"`
	Token string           `json:"token,omitempty"`
}

func (h *SignUpHandler) SignUp(c *gin.Context) {
	var req SignUpRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	user, token, err := h.signUpService.SignUp(c.Request.Context(), biz.SignUpInput{
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
	})
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, SignUpResponse{
		User:  biz.ConvertUserToUserInfo(c.Request.Context(), user),
		Token: token,
	})
}
```

- [ ] **Step 3: Register route in routes.go**

In `internal/server/routes.go`, add the sign-up route in the public (unauthenticated) section, alongside the existing `/admin/auth/signin` route:

```go
// Public auth routes
authGroup := r.Group("/admin/auth")
{
    authGroup.POST("/signin", h.auth.SignIn)
    authGroup.POST("/signup", h.signUp.SignUp) // NEW
}
```

Also register the `SignUpHandler` in the handler struct and wire it in the FX module.

- [ ] **Step 4: Commit**

```bash
git add internal/server/biz/signup.go internal/server/api/signup.go internal/server/routes.go
git commit -m "feat(auth): add user self-registration endpoint"
```

---

### Task 8: Implement PublishRequest business logic

**Files:**
- Create: `internal/server/biz/publish_request.go`

- [ ] **Step 1: Write publish request service**

Create `internal/server/biz/publish_request.go`:

```go
package biz

import (
	"context"
	"fmt"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/publishrequest"
	"github.com/ldm2060/axonhub/internal/ent/schema/schematype"
)

type PublishRequestService struct {
	client *ent.Client
}

func NewPublishRequestService(client *ent.Client) *PublishRequestService {
	return &PublishRequestService{client: client}
}

func (s *PublishRequestService) entFromContext(ctx context.Context) *ent.Client {
	return s.client
}

func (s *PublishRequestService) CreatePublishRequest(ctx context.Context, resourceType, resourceID, requesterID int, comment string) (*ent.PublishRequest, error) {
	// Check for existing pending request
	existing, err := s.entFromContext(ctx).PublishRequest.Query().
		Where(
			publishrequest.ResourceTypeEQ(schematype.PublishRequestResourceType(resourceType)),
			publishrequest.ResourceIDEQ(resourceID),
			publishrequest.RequesterIDEQ(requesterID),
			publishrequest.StatusEQ(publishrequest.StatusPending),
		).First(ctx)
	if err == nil && existing != nil {
		return nil, fmt.Errorf("a pending publish request already exists for this resource")
	}

	req, err := s.entFromContext(ctx).PublishRequest.Create().
		SetResourceType(schematype.PublishRequestResourceType(resourceType)).
		SetResourceID(resourceID).
		SetRequesterID(requesterID).
		SetRequestComment(comment).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create publish request: %w", err)
	}
	return req, nil
}

func (s *PublishRequestService) CancelPublishRequest(ctx context.Context, id int, requesterID int) error {
	req, err := s.entFromContext(ctx).PublishRequest.Get(ctx, id)
	if err != nil {
		return fmt.Errorf("publish request not found: %w", err)
	}
	if req.RequesterID != requesterID {
		return fmt.Errorf("only the requester can cancel")
	}
	if req.Status != publishrequest.StatusPending {
		return fmt.Errorf("only pending requests can be cancelled")
	}
	return s.entFromContext(ctx).PublishRequest.DeleteOneID(id).Exec(ctx)
}

func (s *PublishRequestService) ReviewPublishRequest(ctx context.Context, id int, reviewerID int, action publishrequest.Status, comment string) (*ent.PublishRequest, error) {
	req, err := s.entFromContext(ctx).PublishRequest.Get(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("publish request not found: %w", err)
	}
	if req.Status != publishrequest.StatusPending {
		return nil, fmt.Errorf("only pending requests can be reviewed")
	}

	update := s.entFromContext(ctx).PublishRequest.UpdateOneID(id).
		SetStatus(action).
		SetReviewerID(reviewerID).
		SetReviewComment(comment)

	req, err = update.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update publish request: %w", err)
	}

	// On approval, update the resource visibility
	if action == publishrequest.StatusApproved {
		if err := s.publishResource(ctx, req.ResourceType, req.ResourceID); err != nil {
			return nil, fmt.Errorf("failed to publish resource: %w", err)
		}
	}

	return req, nil
}

func (s *PublishRequestService) publishResource(ctx context.Context, resourceType schematype.PublishRequestResourceType, resourceID int) error {
	switch resourceType {
	case schematype.PublishRequestResourceTypeChannel:
		return s.entFromContext(ctx).Channel.UpdateOneID(resourceID).
			SetVisibility("published").
			ClearSharedWith().
			Exec(ctx)
	case schematype.PublishRequestResourceTypeModel:
		return s.entFromContext(ctx).Model.UpdateOneID(resourceID).
			SetVisibility("published").
			ClearSharedWith().
			Exec(ctx)
	default:
		return fmt.Errorf("unknown resource type: %s", resourceType)
	}
}

func (s *PublishRequestService) ListPublishRequests(ctx context.Context, status *publishrequest.Status) ([]*ent.PublishRequest, error) {
	query := s.entFromContext(ctx).PublishRequest.Query()
	if status != nil {
		query = query.Where(publishrequest.StatusEQ(*status))
	}
	return query.All(ctx)
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/server/biz/publish_request.go
git commit -m "feat(biz): add PublishRequest service with create, review, cancel"
```

---

### Task 9: Add GraphQL schema for publish requests, sharing, and sign-up

**Files:**
- Create: `internal/server/gql/publish_request.graphql`
- Modify: `internal/server/gql/axonhub.graphql`

- [ ] **Step 1: Write publish_request.graphql**

Create `internal/server/gql/publish_request.graphql`:

```graphql
extend type Mutation {
  requestPublish(resourceType: ResourceType!, resourceID: ID!, comment: String): PublishRequest!
  cancelPublishRequest(id: ID!): Boolean!
  reviewPublishRequest(id: ID!, action: ReviewAction!, comment: String): PublishRequest!
  shareChannel(id: ID!, userIDs: [ID!]!): Channel!
  unshareChannel(id: ID!, userIDs: [ID!]!): Channel!
  shareModel(id: ID!, userIDs: [ID!]!): Model!
  unshareModel(id: ID!, userIDs: [ID!]!): Model!
  signUp(input: SignUpInput!): SignUpPayload!
}

extend type Query {
  publishRequests(status: PublishRequestStatus, resourceType: ResourceType): [PublishRequest!]!
  mySharedChannels: [Channel!]!
  mySharedModels: [Model!]!
}

type PublishRequest {
  id: ID!
  resourceType: ResourceType!
  resourceID: Int!
  requester: User!
  status: PublishRequestStatus!
  reviewer: User
  reviewComment: String
  requestComment: String
  createdAt: Time!
  updatedAt: Time!
}

enum ResourceType {
  channel
  model
}

enum ReviewAction {
  approve
  reject
}

enum PublishRequestStatus {
  pending
  approved
  rejected
}

enum Visibility {
  private
  shared
  published
}

input SignUpInput {
  email: String!
  password: String!
  firstName: String
  lastName: String
}

type SignUpPayload {
  user: User!
  token: String
}
```

- [ ] **Step 2: Add Channel/Model visibility fields to axonhub.graphql**

In `axonhub.graphql`, add to the Channel type (or extend type Channel if using schema extension):

```graphql
extend type Channel {
  visibility: Visibility!
  sharedWith: [Int!]
}
```

And similarly for Model:

```graphql
extend type Model {
  visibility: Visibility!
  sharedWith: [Int!]
}
```

- [ ] **Step 3: Regenerate GraphQL code**

```bash
cd D:/PythonProject/axonhub && go generate ./internal/server/gql
```

- [ ] **Step 4: Commit**

```bash
git add internal/server/gql/publish_request.graphql internal/server/gql/axonhub.graphql internal/server/gql/
git commit -m "feat(gql): add publish request, sharing, and sign-up GraphQL schema"
```

---

### Task 10: Implement GraphQL resolvers for sharing and publishing

**Files:**
- Create: `internal/server/gql/publish_request.resolvers.go`
- Modify: `internal/server/gql/channel.resolvers.go`

- [ ] **Step 1: Write publish request resolvers**

Create `internal/server/gql/publish_request.resolvers.go`:

```go
package gql

import (
	"context"
	"fmt"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent/publishrequest"
	"github.com/ldm2060/axonhub/internal/objects"
)

func (r *mutationResolver) RequestPublish(ctx context.Context, resourceType string, resourceID string, comment *string) (*ent.PublishRequest, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return nil, fmt.Errorf("unauthorized")
	}
	resourceIDInt := objects.ExtractNumberIDAsNumber(resourceID)
	return r.publishRequestService.CreatePublishRequest(ctx, resourceType, resourceIDInt, user.ID, derefString(comment))
}

func (r *mutationResolver) CancelPublishRequest(ctx context.Context, id string) (bool, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return false, fmt.Errorf("unauthorized")
	}
	idInt := objects.ExtractNumberIDAsNumber(id)
	return true, r.publishRequestService.CancelPublishRequest(ctx, idInt, user.ID)
}

func (r *mutationResolver) ReviewPublishRequest(ctx context.Context, id string, action string, comment *string) (*ent.PublishRequest, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return nil, fmt.Errorf("unauthorized")
	}
	idInt := objects.ExtractNumberIDAsNumber(id)
	status := publishrequest.StatusApproved
	if action == "reject" {
		status = publishrequest.StatusRejected
	}
	return r.publishRequestService.ReviewPublishRequest(ctx, idInt, user.ID, status, derefString(comment))
}

func (r *queryResolver) PublishRequests(ctx context.Context, status *string, resourceType *string) ([]*ent.PublishRequest, error) {
	var statusFilter *publishrequest.Status
	if status != nil {
		s := publishrequest.Status(*status)
		statusFilter = &s
	}
	return r.publishRequestService.ListPublishRequests(ctx, statusFilter)
}

func (r *queryResolver) MySharedChannels(ctx context.Context) ([]*ent.Channel, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return nil, fmt.Errorf("unauthorized")
	}
	return r.channelService.ListSharedWithUser(ctx, user.ID)
}

func (r *queryResolver) MySharedModels(ctx context.Context) ([]*ent.Model, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return nil, fmt.Errorf("unauthorized")
	}
	return r.modelService.ListSharedWithUser(ctx, user.ID)
}

func derefString(s string) string {
	return s
}
```

- [ ] **Step 2: Add share/unshare resolvers to channel.resolvers.go**

Add to the existing channel resolvers file:

```go
func (r *mutationResolver) ShareChannel(ctx context.Context, id string, userIDs []string) (*ent.Channel, error) {
	currentUser, ok := contexts.GetUser(ctx)
	if !ok || currentUser == nil {
		return nil, fmt.Errorf("unauthorized")
	}
	idInt := objects.ExtractNumberIDAsNumber(id)
	return r.channelService.ShareChannel(ctx, idInt, currentUser.ID, objects.IntGuids(userIDs))
}

func (r *mutationResolver) UnshareChannel(ctx context.Context, id string, userIDs []string) (*ent.Channel, error) {
	currentUser, ok := contexts.GetUser(ctx)
	if !ok || currentUser == nil {
		return nil, fmt.Errorf("unauthorized")
	}
	idInt := objects.ExtractNumberIDAsNumber(id)
	return r.channelService.UnshareChannel(ctx, idInt, currentUser.ID, objects.IntGuids(userIDs))
}

func (r *mutationResolver) ShareModel(ctx context.Context, id string, userIDs []string) (*ent.Model, error) {
	currentUser, ok := contexts.GetUser(ctx)
	if !ok || currentUser == nil {
		return nil, fmt.Errorf("unauthorized")
	}
	idInt := objects.ExtractNumberIDAsNumber(id)
	return r.modelService.ShareModel(ctx, idInt, currentUser.ID, objects.IntGuids(userIDs))
}

func (r *mutationResolver) UnshareModel(ctx context.Context, id string, userIDs []string) (*ent.Model, error) {
	currentUser, ok := contexts.GetUser(ctx)
	if !ok || currentUser == nil {
		return nil, fmt.Errorf("unauthorized")
	}
	idInt := objects.ExtractNumberIDAsNumber(id)
	return r.modelService.UnshareModel(ctx, idInt, currentUser.ID, objects.IntGuids(userIDs))
}

func (r *mutationResolver) SignUp(ctx context.Context, input SignUpInput) (*SignUpPayload, error) {
	user, token, err := r.signUpService.SignUp(ctx, biz.SignUpInput{
		Email:     input.Email,
		Password:  input.Password,
		FirstName: input.FirstName,
		LastName:  input.LastName,
	})
	if err != nil {
		return nil, err
	}
	return &SignUpPayload{
		User:  biz.ConvertUserToUserInfo(ctx, user),
		Token: &token,
	}, nil
}
```

- [ ] **Step 3: Add ShareChannel/UnshareChannel methods to channel biz service**

In `internal/server/biz/channel.go`, add:

```go
func (svc *ChannelService) ShareChannel(ctx context.Context, channelID, ownerID int, userIDs []int) (*ent.Channel, error) {
	ch, err := svc.entFromContext(ctx).Channel.Get(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}
	if ch.OwnerID == nil || *ch.OwnerID != ownerID {
		return nil, fmt.Errorf("only the owner can share this channel")
	}

	sharedWith := ch.SharedWith
	for _, uid := range userIDs {
		if uid == ownerID {
			continue
		}
		if !slices.Contains(sharedWith, uid) {
			sharedWith = append(sharedWith, uid)
		}
	}

	ch, err = svc.entFromContext(ctx).Channel.UpdateOneID(channelID).
		SetSharedWith(sharedWith).
		SetVisibility("shared").
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to share channel: %w", err)
	}
	svc.asyncReloadChannels()
	return ch, nil
}

func (svc *ChannelService) UnshareChannel(ctx context.Context, channelID, ownerID int, userIDs []int) (*ent.Channel, error) {
	ch, err := svc.entFromContext(ctx).Channel.Get(ctx, channelID)
	if err != nil {
		return nil, fmt.Errorf("channel not found: %w", err)
	}
	if ch.OwnerID == nil || *ch.OwnerID != ownerID {
		return nil, fmt.Errorf("only the owner can unshare this channel")
	}

	sharedWith := lo.Filter(ch.SharedWith, func(id int, _ int) bool {
		return !slices.Contains(userIDs, id)
	})

	visibility := "shared"
	if len(sharedWith) == 0 {
		visibility = "private"
	}

	ch, err = svc.entFromContext(ctx).Channel.UpdateOneID(channelID).
		SetSharedWith(sharedWith).
		SetVisibility(visibility).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to unshare channel: %w", err)
	}
	svc.asyncReloadChannels()
	return ch, nil
}

func (svc *ChannelService) ListSharedWithUser(ctx context.Context, userID int) ([]*ent.Channel, error) {
	// Application-level filter for shared_with JSON array containment
	channels, err := svc.entFromContext(ctx).Channel.Query().
		Where(channel.VisibilityEQ("shared")).
		All(ctx)
	if err != nil {
		return nil, err
	}
	return lo.Filter(channels, func(ch *ent.Channel, _ int) bool {
		return isSharedWithUser(ch.SharedWith, userID)
	}), nil
}
```

Add analogous `ShareModel`, `UnshareModel`, `ListSharedWithUser` methods to `internal/server/biz/model.go`.

- [ ] **Step 4: Set owner_id on channel/model creation**

In `CreateChannel`, add:

```go
if currentUser, ok := contexts.GetUser(ctx); ok && currentUser != nil {
    createBuilder = createBuilder.SetOwnerID(currentUser.ID)
}
```

Same pattern in `CreateModel` (or the model creation equivalent).

- [ ] **Step 5: Commit**

```bash
git add internal/server/gql/publish_request.resolvers.go internal/server/gql/channel.resolvers.go internal/server/biz/channel.go internal/server/biz/model.go
git commit -m "feat(gql): add share/unshare/publish resolvers and biz methods"
```

---

### Task 11: Add user-scoped dashboard resolver

**Files:**
- Modify: `internal/server/gql/dashboard.resolvers.go`

- [ ] **Step 1: Add myDashboard query resolver**

Add a new resolver that filters all dashboard queries by the current user's `private_project_id`:

```go
func (r *queryResolver) MyDashboard(ctx context.Context) (*DashboardOverview, error) {
	user, ok := contexts.GetUser(ctx)
	if !ok || user == nil {
		return nil, fmt.Errorf("unauthorized")
	}
	projectID, err := r.userService.EnsurePrivateProject(ctx, user)
	if err != nil {
		return nil, err
	}
	// Inject project ID into context so all downstream queries filter by it
	ctx = contexts.WithProjectID(ctx, projectID)
	// Reuse existing dashboard logic, which now respects the project_id filter
	return r.DashboardOverview(ctx)
}
```

Modify existing dashboard resolvers to respect `project_id` from context when present. For each resolver that queries `r.client.Request.Query()` or `r.client.UsageLog.Query()`, add:

```go
if projectID, ok := contexts.GetProjectID(ctx); ok {
    query = query.Where(request.ProjectIDEQ(projectID))
}
```

The global dashboard (`/admin/dashboard`) continues to work without a project_id in context, showing all data.

- [ ] **Step 2: Commit**

```bash
git add internal/server/gql/dashboard.resolvers.go
git commit -m "feat(dashboard): add user-scoped myDashboard resolver"
```

---

### Task 12: Adapt OIDC JIT provisioning

**Files:**
- Modify: `internal/server/biz/oidc.go`

- [ ] **Step 1: Add allow_sign_up check to resolveUser**

In the OIDC `resolveUser` function, when reaching Step 3 (JIT provisioning), add a check:

```go
allowSignUp, _ := s.systemService.GetBoolSetting(ctx, "allow_sign_up")
if !allowSignUp {
    return nil, fmt.Errorf("automatic user creation is disabled; contact an administrator")
}
```

- [ ] **Step 2: Set default user scopes on JIT-created users**

When creating the user via JIT, set `Scopes: DefaultUserScopes` instead of empty.

- [ ] **Step 3: Commit**

```bash
git add internal/server/biz/oidc.go
git commit -m "feat(oidc): respect allow_sign_up setting in JIT provisioning"
```

---

### Task 13: Add data migration for existing data

**Files:**
- Modify: `internal/ent/migrate/datamigrate/migrator.go`

- [ ] **Step 1: Add migration steps**

Add a new migration function that runs after schema migration:

```go
func (m *Migrator) migrateUserLevelManagement(ctx context.Context) error {
	client := m.client

	// Step 1: Set all existing channels as system-owned and published
	_, err := client.Channel.Update().
		Where(channel.OwnerIDIsNil()).
		SetVisibility("published").
		Save(ctx)
	if err != nil {
		return fmt.Errorf("migrate channels visibility: %w", err)
	}

	// Step 2: Set all existing models as system-owned and published
	_, err = client.Model.Update().
		Where(model.OwnerIDIsNil()).
		SetVisibility("published").
		Save(ctx)
	if err != nil {
		return fmt.Errorf("migrate models visibility: %w", err)
	}

	// Step 3: For each user without a private project, create one
	users, err := client.User.Query().All(ctx)
	if err != nil {
		return fmt.Errorf("query users: %w", err)
	}
	for _, u := range users {
		if u.PrivateProjectID != nil {
			continue
		}
		projectID, err := m.projectService.CreatePrivateProject(ctx, u.ID)
		if err != nil {
			return fmt.Errorf("create private project for user %d: %w", u.ID, err)
		}
		client.User.UpdateOneID(u.ID).
			SetPrivateProjectID(projectID).
			Save(ctx)
	}

	// Step 4: Migrate default project API keys to per-user private projects
	apiKeys, err := client.APIKey.Query().
		Where(apikey.ProjectIDEQ(1)).
		All(ctx)
	if err != nil {
		return fmt.Errorf("query default project api keys: %w", err)
	}
	for _, key := range apiKeys {
		if key.UserID == nil {
			continue
		}
		owner, err := client.User.Get(ctx, *key.UserID)
		if err != nil || owner.PrivateProjectID == nil {
			continue
		}
		client.APIKey.UpdateOneID(key.ID).
			SetProjectID(*owner.PrivateProjectID).
			Save(ctx)
	}

	return nil
}
```

- [ ] **Step 2: Commit**

```bash
git add internal/ent/migrate/datamigrate/migrator.go
git commit -m "feat(migrate): add user-level management data migration"
```

---

### Task 14: Frontend — Remove project UI and auto-resolve project context

**Files:**
- Modify: `frontend/src/stores/projectStore.ts`
- Delete: `frontend/src/components/layout/project-switcher.tsx`
- Delete: `frontend/src/components/project-guard.tsx`
- Modify: All route files under `frontend/src/routes/_authenticated/project/`

- [ ] **Step 1: Replace projectStore with auto-resolution**

Replace the contents of `frontend/src/stores/projectStore.ts`:

```ts
import { create } from 'zustand';
import { useMe } from '@/features/user/data/user';

// No longer user-selectable — resolved from current user's privateProjectId
export function usePrivateProjectId(): number | null {
  const { data: me } = useMe();
  return me?.privateProjectID ?? null;
}
```

- [ ] **Step 2: Delete ProjectSwitcher and ProjectGuard**

```bash
rm frontend/src/components/layout/project-switcher.tsx
rm frontend/src/components/project-guard.tsx
```

- [ ] **Step 3: Replace all `useSelectedProjectId()` calls**

Search all files for `useSelectedProjectId` and replace with `usePrivateProjectId()`. The return type changes from `string | null` to `number | null`, so also update any `X-Project-ID` header construction:

```ts
// Before
const selectedProjectId = useSelectedProjectId();
const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;

// After
const privateProjectId = usePrivateProjectId();
const headers = privateProjectId ? { 'X-Project-ID': String(privateProjectId) } : undefined;
```

- [ ] **Step 4: Remove ProjectGuard wrappers from route files**

In all route files under `frontend/src/routes/_authenticated/project/`, remove `<ProjectGuard>` wrapper components. The routes themselves remain functional — they just no longer need a guard since the project is auto-resolved.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/stores/projectStore.ts frontend/src/components/ frontend/src/routes/
git commit -m "feat(frontend): remove project selector, auto-resolve private project"
```

---

### Task 15: Frontend — Restructure sidebar navigation

**Files:**
- Modify: `frontend/src/sidebar.ts`

- [ ] **Step 1: Rewrite sidebar for normal users**

Replace the sidebar structure. The `useSidebarData()` function should return different nav groups based on user role:

```ts
export function useSidebarData() {
  const { t } = useTranslation();
  const { data: me } = useMe();
  const isAdmin = hasScope(me, 'write_channels');
  const isOwner = me?.isOwner ?? false;

  const personalItems: NavItem[] = [
    { url: '/dashboard', icon: BarChart3Icon, title: t('sidebar.myDashboard') },
    { url: '/channels', icon: ChannelIcon, title: t('sidebar.myChannels') },
    { url: '/models', icon: ModelIcon, title: t('sidebar.myModels') },
    { url: '/api-keys', icon: KeyIcon, title: t('sidebar.apiKeys') },
    { url: '/prompts', icon: PromptIcon, title: t('sidebar.prompts') },
    { url: '/requests', icon: RequestIcon, title: t('sidebar.requests') },
    { url: '/traces', icon: TraceIcon, title: t('sidebar.traces') },
    { url: '/threads', icon: ThreadIcon, title: t('sidebar.threads') },
    { url: '/playground', icon: PlaygroundIcon, title: t('sidebar.playground') },
  ];

  const discoverItems: NavItem[] = [
    { url: '/discover/channels', icon: GlobeIcon, title: t('sidebar.publicChannels') },
    { url: '/discover/models', icon: GlobeIcon, title: t('sidebar.publicModels') },
  ];

  const navGroups: NavGroup[] = [
    { title: t('sidebar.groups.personal'), items: personalItems },
    { title: t('sidebar.groups.discover'), items: discoverItems },
  ];

  if (isAdmin || isOwner) {
    const adminItems: NavItem[] = [
      { url: '/admin/dashboard', icon: BarChart3Icon, title: t('sidebar.globalDashboard') },
      { url: '/admin/users', icon: UsersIcon, title: t('sidebar.users') },
      { url: '/admin/review-queue', icon: ClipboardIcon, title: t('sidebar.reviewQueue') },
      { url: '/admin/channels', icon: ChannelIcon, title: t('sidebar.allChannels') },
      { url: '/admin/models', icon: ModelIcon, title: t('sidebar.allModels') },
      { url: '/admin/roles', icon: ShieldIcon, title: t('sidebar.roles') },
      { url: '/system', icon: SettingsIcon, title: t('sidebar.system'), mobileOnly: true },
    ];
    navGroups.push({ title: t('sidebar.groups.admin'), items: adminItems });
  }

  // ... rest of sidebar data construction
}
```

- [ ] **Step 2: Commit**

```bash
git add frontend/src/sidebar.ts
git commit -m "feat(frontend): restructure sidebar for user-level navigation"
```

---

### Task 16: Frontend — Add sign-up page

**Files:**
- Create: `frontend/src/routes/_authenticated/sign-up/index.tsx`

- [ ] **Step 1: Create sign-up page**

Create `frontend/src/routes/_authenticated/sign-up/index.tsx`:

```tsx
import { createFileRoute } from '@tanstack/react-router';
import { SignUpForm } from '@/features/auth/components/sign-up-form';

export const Route = createFileRoute('/_authenticated/sign-up/')({
  component: SignUpPage,
});

function SignUpPage() {
  return (
    <div className="flex min-h-svh flex-col items-center justify-center gap-6 p-6 md:p-10">
      <div className="flex w-full max-w-sm flex-col gap-6">
        <SignUpForm />
      </div>
    </div>
  );
}
```

Create `frontend/src/features/auth/components/sign-up-form.tsx` with a form matching the existing sign-in form pattern, calling `POST /admin/auth/signup`.

- [ ] **Step 2: Add "Sign Up" link to sign-in page**

On the existing sign-in page, add a link: "Don't have an account? Sign up" pointing to `/sign-up`.

- [ ] **Step 3: Commit**

```bash
git add frontend/src/routes/_authenticated/sign-up/ frontend/src/features/auth/
git commit -m "feat(frontend): add sign-up page"
```

---

### Task 17: Frontend — Add channel/model sharing and publish UI

**Files:**
- Modify: `frontend/src/features/channels/data/channels.ts`
- Create: `frontend/src/features/channels/components/channel-share-dialog.tsx`
- Modify: `frontend/src/features/channels/components/channels-action-dialog.tsx`
- Modify: `frontend/src/features/models/data/models.ts`
- Create: `frontend/src/features/models/components/model-share-dialog.tsx`

- [ ] **Step 1: Add GraphQL hooks for sharing/publishing**

In `channels.ts`, add:

```ts
const SHARE_CHANNEL = gql`
  mutation ShareChannel($id: ID!, $userIDs: [ID!]!) {
    shareChannel(id: $id, userIDs: $userIDs) { id visibility sharedWith }
  }
`;

const UNSHARE_CHANNEL = gql`
  mutation UnshareChannel($id: ID!, $userIDs: [ID!]!) {
    unshareChannel(id: $id, userIDs: $userIDs) { id visibility sharedWith }
  }
`;

const REQUEST_PUBLISH = gql`
  mutation RequestPublish($resourceType: ResourceType!, $resourceID: ID!, $comment: String) {
    requestPublish(resourceType: $resourceType, resourceID: $resourceID, comment: $comment) { id status }
  }
`;

export function useShareChannel() {
  return useMutation({
    mutationFn: (vars: { id: string; userIDs: string[] }) =>
      graphqlRequest(SHARE_CHANNEL, vars),
  });
}

export function useUnshareChannel() {
  return useMutation({
    mutationFn: (vars: { id: string; userIDs: string[] }) =>
      graphqlRequest(UNSHARE_CHANNEL, vars),
  });
}

export function useRequestPublish() {
  return useMutation({
    mutationFn: (vars: { resourceType: string; resourceID: string; comment?: string }) =>
      graphqlRequest(REQUEST_PUBLISH, vars),
  });
}
```

Add analogous hooks in `models.ts`.

- [ ] **Step 2: Create share dialog component**

Create `frontend/src/features/channels/components/channel-share-dialog.tsx`:

A dialog with:
- User search input (queries `users` with name/email filter)
- List of currently shared users with remove buttons
- "Share" button to add selected users
- Visibility badge showing current state (private/shared)

- [ ] **Step 3: Add share/publish buttons to channel detail**

In the channel action dialog (edit view), add:
- "Share" button → opens ChannelShareDialog
- "Request Public Publishing" button → calls `useRequestPublish` with `resourceType: "channel"`

- [ ] **Step 4: Add visibility badge to channel/model table rows**

In the channels table columns, add a badge column showing the visibility state with appropriate colors:
- `private` → gray
- `shared` → yellow
- `published` → green

- [ ] **Step 5: Repeat steps 2-4 for models**

- [ ] **Step 6: Commit**

```bash
git add frontend/src/features/channels/ frontend/src/features/models/
git commit -m "feat(frontend): add channel/model sharing and publish request UI"
```

---

### Task 18: Frontend — Add admin review queue page

**Files:**
- Create: `frontend/src/features/publish-requests/index.tsx`
- Create: `frontend/src/features/publish-requests/data/publish-requests.ts`
- Create: `frontend/src/features/publish-requests/components/publish-request-card.tsx`

- [ ] **Step 1: Add GraphQL hooks**

Create `frontend/src/features/publish-requests/data/publish-requests.ts`:

```ts
const PUBLISH_REQUESTS = gql`
  query PublishRequests($status: PublishRequestStatus) {
    publishRequests(status: $status) {
      id resourceType resourceID status requestComment reviewComment
      requester { id email firstName lastName }
      reviewer { id email firstName lastName }
      createdAt updatedAt
    }
  }
`;

const REVIEW_PUBLISH_REQUEST = gql`
  mutation ReviewPublishRequest($id: ID!, $action: ReviewAction!, $comment: String) {
    reviewPublishRequest(id: $id, action: $action, comment: $comment) { id status }
  }
`;

export function usePublishRequests(status?: string) {
  return useQuery({
    queryKey: ['publishRequests', status],
    queryFn: () => graphqlRequest(PUBLISH_REQUESTS, { status }),
  });
}

export function useReviewPublishRequest() {
  return useMutation({
    mutationFn: (vars: { id: string; action: string; comment?: string }) =>
      graphqlRequest(REVIEW_PUBLISH_REQUEST, vars),
  });
}
```

- [ ] **Step 2: Create review queue page**

Create `frontend/src/features/publish-requests/index.tsx` with:
- Status filter tabs (pending / approved / rejected)
- List of publish request cards
- Each card shows: resource type badge, resource name (linked), requester info, request comment, approve/reject buttons with optional comment

- [ ] **Step 3: Create review card component**

Create `frontend/src/features/publish-requests/components/publish-request-card.tsx`:
- Shows resource type (channel/model) with icon
- Shows requester name and email
- Shows request comment
- Approve button (green) and Reject button (red)
- Optional comment input for review

- [ ] **Step 4: Add route**

Create `frontend/src/routes/_authenticated/admin/review-queue/index.tsx`:

```tsx
import { createFileRoute } from '@tanstack/react-router';
import PublishRequestsPage from '@/features/publish-requests';

export const Route = createFileRoute('/_authenticated/admin/review-queue/')({
  component: PublishRequestsPage,
});
```

- [ ] **Step 5: Commit**

```bash
git add frontend/src/features/publish-requests/ frontend/src/routes/_authenticated/admin/
git commit -m "feat(frontend): add admin publish request review queue"
```

---

### Task 19: Frontend — Add dual dashboards

**Files:**
- Modify: `frontend/src/features/dashboard/index.tsx`
- Create: `frontend/src/features/my-dashboard/index.tsx`
- Create: `frontend/src/routes/_authenticated/dashboard/index.tsx` (update)
- Create: `frontend/src/routes/_authenticated/admin/dashboard/index.tsx`

- [ ] **Step 1: Create my-dashboard page**

Create `frontend/src/features/my-dashboard/index.tsx` — a copy of the existing dashboard page but with the `X-Project-ID` header set to the current user's `privateProjectId`. All existing dashboard widgets are reused.

- [ ] **Step 2: Rename existing dashboard to admin-dashboard**

Rename `frontend/src/features/dashboard/index.tsx` to serve as the global admin dashboard. No `X-Project-ID` header is sent (shows all data).

- [ ] **Step 3: Update routes**

Update `frontend/src/routes/_authenticated/dashboard/index.tsx` to render `MyDashboard`.
Create `frontend/src/routes/_authenticated/admin/dashboard/index.tsx` to render the global admin dashboard (with route guard for admin only).

- [ ] **Step 4: Commit**

```bash
git add frontend/src/features/dashboard/ frontend/src/features/my-dashboard/ frontend/src/routes/
git commit -m "feat(frontend): add dual dashboards (my + global admin)"
```

---

### Task 20: Frontend — Add i18n keys

**Files:**
- Modify: `frontend/src/locales/en.json`
- Modify: `frontend/src/locales/zh.json`

- [ ] **Step 1: Add English keys**

Add to `en.json`:

```json
{
  "sidebar": {
    "myDashboard": "My Dashboard",
    "myChannels": "My Channels",
    "myModels": "My Models",
    "publicChannels": "Public Channels",
    "publicModels": "Public Models",
    "globalDashboard": "Global Dashboard",
    "reviewQueue": "Review Queue",
    "allChannels": "All Channels",
    "allModels": "All Models",
    "groups": {
      "personal": "Personal",
      "discover": "Discover",
      "admin": "Admin"
    }
  },
  "visibility": {
    "private": "Private",
    "shared": "Shared",
    "published": "Published"
  },
  "sharing": {
    "share": "Share",
    "unshare": "Remove",
    "sharedWithMe": "Shared with me",
    "searchUsers": "Search users...",
    "noUsers": "No users shared"
  },
  "publish": {
    "request": "Request Public Publishing",
    "cancel": "Cancel Request",
    "pending": "Pending Review",
    "approved": "Approved",
    "rejected": "Rejected",
    "comment": "Comment (optional)",
    "approve": "Approve",
    "reject": "Reject"
  },
  "signUp": {
    "title": "Create Account",
    "submit": "Sign Up",
    "disabled": "Registration is currently closed",
    "link": "Don't have an account? Sign up"
  },
  "dashboard": {
    "myTitle": "My Dashboard",
    "globalTitle": "Global Dashboard"
  }
}
```

- [ ] **Step 2: Add Chinese keys**

Add corresponding entries to `zh.json`:

```json
{
  "sidebar": {
    "myDashboard": "我的仪表盘",
    "myChannels": "我的渠道",
    "myModels": "我的模型",
    "publicChannels": "公共渠道",
    "publicModels": "公共模型",
    "globalDashboard": "全局仪表盘",
    "reviewQueue": "审核队列",
    "allChannels": "全部渠道",
    "allModels": "全部模型",
    "groups": {
      "personal": "个人空间",
      "discover": "发现",
      "admin": "全局管理"
    }
  },
  "visibility": {
    "private": "私有",
    "shared": "已分享",
    "published": "已发布"
  },
  "sharing": {
    "share": "分享",
    "unshare": "移除",
    "sharedWithMe": "与我分享的",
    "searchUsers": "搜索用户...",
    "noUsers": "暂无分享用户"
  },
  "publish": {
    "request": "申请公开发布",
    "cancel": "取消申请",
    "pending": "待审核",
    "approved": "已通过",
    "rejected": "已拒绝",
    "comment": "备注（可选）",
    "approve": "通过",
    "reject": "拒绝"
  },
  "signUp": {
    "title": "创建账户",
    "submit": "注册",
    "disabled": "注册功能已关闭",
    "link": "没有账户？注册"
  },
  "dashboard": {
    "myTitle": "我的仪表盘",
    "globalTitle": "全局仪表盘"
  }
}
```

- [ ] **Step 3: Commit**

```bash
git add frontend/src/locales/en.json frontend/src/locales/zh.json
git commit -m "feat(i18n): add keys for user-level management"
```

---

### Task 21: Backend — Set owner_id on channel/model creation and resolve project_id from user context

**Files:**
- Modify: `internal/server/biz/channel.go`
- Modify: `internal/server/biz/model.go`
- Modify: `internal/contexts/context.go` (if needed for auto-injection)

- [ ] **Step 1: Auto-inject project_id from user context**

Add a middleware or context helper that, when no `X-Project-ID` header is present, automatically sets the project_id from the authenticated user's `private_project_id`:

In the auth middleware (or a new middleware), after authenticating the user:

```go
if projectID, ok := contexts.GetProjectID(ctx); !ok {
    if user, ok := contexts.GetUser(ctx); ok && user != nil && user.PrivateProjectID != nil {
        ctx = contexts.WithProjectID(ctx, *user.PrivateProjectID)
    }
}
```

This ensures all existing project-scoped logic works without requiring the frontend to send `X-Project-ID`.

- [ ] **Step 2: Set owner_id on channel creation**

In `CreateChannel`, after building the create builder, add:

```go
if currentUser, ok := contexts.GetUser(ctx); ok && currentUser != nil {
    createBuilder = createBuilder.SetOwnerID(currentUser.ID)
}
```

- [ ] **Step 3: Set owner_id on model creation**

Same pattern in the model creation code.

- [ ] **Step 4: Commit**

```bash
git add internal/server/biz/channel.go internal/server/biz/model.go internal/contexts/
git commit -m "feat(biz): auto-set owner_id and resolve project_id from user context"
```

---

### Task 22: Handle user deletion cleanup

**Files:**
- Modify: `internal/server/biz/user.go`

- [ ] **Step 1: Add cleanup logic to DeleteUser**

In the `DeleteUser` method, before soft-deleting the user, add cleanup for their channels and models:

```go
// Clean up user's channels based on visibility
channels, _ := client.Channel.Query().Where(channel.OwnerIDEQ(user.ID)).All(ctx)
for _, ch := range channels {
    switch ch.Visibility {
    case "private":
        client.Channel.DeleteOneID(ch.ID).Exec(ctx)
    case "shared":
        // Remove user from other resources' shared_with lists
        // (nothing to do — this user IS the owner, not a share target)
        client.Channel.DeleteOneID(ch.ID).Exec(ctx)
    case "published":
        // Reassign to system ownership so other users aren't disrupted
        client.Channel.UpdateOneID(ch.ID).
            ClearOwnerID().
            Save(ctx)
    }
}

// Same for models
models, _ := client.Model.Query().Where(model.OwnerIDEQ(user.ID)).All(ctx)
for _, m := range models {
    switch m.Visibility {
    case "published":
        client.Model.UpdateOneID(m.ID).ClearOwnerID().Save(ctx)
    default:
        client.Model.DeleteOneID(m.ID).Exec(ctx)
    }
}

// Cancel any pending publish requests
client.PublishRequest.Delete().
    Where(publishrequest.RequesterIDEQ(user.ID), publishrequest.StatusEQ(publishrequest.StatusPending)).
    Exec(ctx)
```

- [ ] **Step 2: Commit**

```bash
git add internal/server/biz/user.go
git commit -m "feat(biz): handle channel/model cleanup on user deletion"
```

---

### Task 23: Integration verification

- [ ] **Step 1: Start the dev server and verify the full flow**

1. Register a new user via `/sign-up`
2. Login as the new user
3. Verify: sidebar shows "Personal" and "Discover" groups only
4. Create a channel → verify `owner_id` is set, visibility is `private`
5. Create a model → verify same
6. Share the channel with another user → verify visibility changes to `shared`
7. Request publishing the channel → verify PublishRequest is created
8. Login as admin → verify "Admin" group appears in sidebar
9. Approve the publish request → verify channel visibility changes to `published`
10. Login as the other user → verify the shared channel appears with "Shared with me" tag
11. Verify "My Dashboard" shows only the new user's data
12. Verify "Global Dashboard" (admin) shows all data

- [ ] **Step 2: Final commit**

```bash
git add -A
git commit -m "feat: complete user-level management implementation"
```