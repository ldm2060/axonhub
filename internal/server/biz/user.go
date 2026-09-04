package biz

import (
	"context"
	"fmt"
	"time"

	"github.com/samber/lo"
	"go.uber.org/fx"
	"go.uber.org/zap"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/model"
	"github.com/ldm2060/axonhub/internal/ent/publishrequest"
	"github.com/ldm2060/axonhub/internal/ent/role"
	"github.com/ldm2060/axonhub/internal/ent/user"
	"github.com/ldm2060/axonhub/internal/ent/userproject"
	"github.com/ldm2060/axonhub/internal/ent/userrole"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/internal/pkg/xcache"
)

type UserServiceParams struct {
	fx.In

	CacheConfig    xcache.Config
	Ent            *ent.Client
	APIKeyService  *APIKeyService  `optional:"true"`
	ProjectService *ProjectService `optional:"true"`
	EmailService   *EmailService   `optional:"true"`
	SystemService  *SystemService  `optional:"true"`
}

type UserService struct {
	*AbstractService

	UserCache           xcache.Cache[ent.User]
	apiKeyService       *APIKeyService
	permissionValidator *PermissionValidator
	projectService      *ProjectService
	emailService        *EmailService
	systemService       *SystemService
}

func NewUserService(params UserServiceParams) *UserService {
	return &UserService{
		AbstractService: &AbstractService{
			db: params.Ent,
		},
		UserCache:           xcache.NewFromConfig[ent.User](params.CacheConfig),
		apiKeyService:       params.APIKeyService,
		permissionValidator: NewPermissionValidator(),
		projectService:      params.ProjectService,
		emailService:        params.EmailService,
		systemService:       params.SystemService,
	}
}

// CreateUser creates a new user with hashed password.
func (s *UserService) CreateUser(ctx context.Context, input ent.CreateUserInput) (*ent.User, error) {
	client := s.entFromContext(ctx)

	// Creating a user must not become a way to mint privileges the caller lacks.
	// Self-registration and other internal flows run under a system bypass, which
	// leaves no user in context and therefore skips these checks.
	if _, ok := contexts.GetUser(ctx); ok {
		if input.IsOwner != nil && *input.IsOwner {
			if err := s.permissionValidator.CanGrantOwnership(ctx); err != nil {
				return nil, fmt.Errorf("permission denied: %w", err)
			}
		}

		if input.Scopes != nil {
			if err := s.permissionValidator.CanGrantScopes(ctx, input.Scopes, nil); err != nil {
				return nil, fmt.Errorf("permission denied: %w", err)
			}
		}

		for _, roleID := range input.RoleIDs {
			if err := s.permissionValidator.CanEditRole(ctx, roleID, nil); err != nil {
				return nil, fmt.Errorf("permission denied: %w", err)
			}
		}
	}

	// Hash the password
	var hashedPassword string
	if input.Password == OIDC_ONLY_PLACEHOLDER {
		hashedPassword = OIDC_ONLY_PLACEHOLDER
	} else {
		var err error

		hashedPassword, err = HashPassword(input.Password)
		if err != nil {
			return nil, err
		}
	}

	mut := client.User.Create().
		SetNillableFirstName(input.FirstName).
		SetNillableLastName(input.LastName).
		SetEmail(input.Email).
		SetPassword(hashedPassword).
		SetScopes(input.Scopes)

	if input.RoleIDs != nil {
		mut.AddRoleIDs(input.RoleIDs...)
	}

	user, err := mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Auto-create private project if ProjectService is available
	if s.projectService != nil {
		projectID, err := s.projectService.CreatePrivateProject(ctx, user.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to create private project: %w", err)
		}

		user, err = client.User.UpdateOneID(user.ID).
			SetPrivateProjectID(projectID).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to set private project: %w", err)
		}
	}

	return user, nil
}

// EnsurePrivateProject returns the user's private project ID, creating one if needed.
func (s *UserService) EnsurePrivateProject(ctx context.Context, u *ent.User) (int, error) {
	if u.PrivateProjectID != nil {
		return *u.PrivateProjectID, nil
	}

	if s.projectService == nil {
		return 0, fmt.Errorf("project service not available")
	}

	projectID, err := s.projectService.CreatePrivateProject(ctx, u.ID)
	if err != nil {
		return 0, err
	}

	_, err = s.entFromContext(ctx).User.UpdateOneID(u.ID).
		SetPrivateProjectID(projectID).
		Save(ctx)
	if err != nil {
		return 0, err
	}

	return projectID, nil
}

// UpdateSelf updates the current user's own profile fields (firstName, lastName, preferLanguage).
// No permission check is needed since a user always has permission to update their own profile.
// Bypasses Ent privacy layer since this is a validated self-service operation on safe fields only.
func (s *UserService) UpdateSelf(ctx context.Context, id int, input ent.UpdateUserInput) (*ent.User, error) {
	return authz.RunWithSystemBypass(ctx, "update-self-profile", func(bypassCtx context.Context) (*ent.User, error) {
		client := s.entFromContext(bypassCtx)

		user, err := client.User.UpdateOneID(id).
			SetNillableFirstName(input.FirstName).
			SetNillableLastName(input.LastName).
			SetNillablePreferLanguage(input.PreferLanguage).
			Save(bypassCtx)
		if err != nil {
			return nil, fmt.Errorf("failed to update profile: %w", err)
		}

		// Invalidate cache
		s.invalidateUserCache(ctx, id)

		return user, nil
	})
}

// UpdateSelfPassword updates the current user's own password.
// No permission check is needed since a user always has permission to update their own password.
// Bypasses Ent privacy layer since this is a validated self-service operation.
func (s *UserService) UpdateSelfPassword(ctx context.Context, id int, newPassword string) error {
	return authz.RunWithSystemBypassVoid(ctx, "update-self-password", func(bypassCtx context.Context) error {
		hashedPassword, err := HashPassword(newPassword)
		if err != nil {
			return err
		}

		_, err = s.entFromContext(bypassCtx).User.UpdateOneID(id).
			SetPassword(hashedPassword).
			Save(bypassCtx)
		if err != nil {
			return fmt.Errorf("failed to update password: %w", err)
		}

		// Invalidate cache
		s.invalidateUserCache(ctx, id)

		return nil
	})
}

// UpdateUser updates an existing user (admin operation).
// Validates permissions before updating scopes, roles, and other sensitive fields.
func (s *UserService) UpdateUser(ctx context.Context, id int, input ent.UpdateUserInput) (*ent.User, error) {
	// Validate permissions before updating
	if err := s.permissionValidator.CanEditUserPermissions(ctx, id, nil); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// is_owner is not a scope, so CanGrantScopes never covers it. Without this guard a
	// holder of write_users could grant ownership to themselves and bypass every policy.
	if input.IsOwner != nil {
		if err := s.permissionValidator.CanGrantOwnership(ctx); err != nil {
			return nil, fmt.Errorf("permission denied: %w", err)
		}
	}

	// Validate scope grants if scopes are being updated
	if input.Scopes != nil {
		if err := s.permissionValidator.CanGrantScopes(ctx, input.Scopes, nil); err != nil {
			return nil, fmt.Errorf("permission denied: %w", err)
		}
	}

	// Validate scope grants when appending rather than replacing.
	if input.AppendScopes != nil {
		if err := s.permissionValidator.CanGrantScopes(ctx, input.AppendScopes, nil); err != nil {
			return nil, fmt.Errorf("permission denied: %w", err)
		}
	}

	// Validate role grants if roles are being added
	if input.AddRoleIDs != nil {
		for _, roleID := range input.AddRoleIDs {
			if err := s.permissionValidator.CanEditRole(ctx, roleID, nil); err != nil {
				return nil, fmt.Errorf("permission denied: %w", err)
			}
		}
	}

	client := s.entFromContext(ctx)

	mut := client.User.UpdateOneID(id).
		SetNillableEmail(input.Email).
		SetNillableFirstName(input.FirstName).
		SetNillableLastName(input.LastName).
		SetNillableIsOwner(input.IsOwner).
		SetNillablePreferLanguage(input.PreferLanguage)

	if input.Password != nil {
		hashedPassword, err := HashPassword(*input.Password)
		if err != nil {
			return nil, err
		}

		mut.SetPassword(hashedPassword)
	}

	if input.Scopes != nil {
		mut.SetScopes(input.Scopes)
	}

	if input.AppendScopes != nil {
		mut.AppendScopes(input.AppendScopes)
	}

	if input.ClearScopes {
		mut.ClearScopes()
	}

	if input.AddRoleIDs != nil {
		mut.AddRoleIDs(input.AddRoleIDs...)
	}

	if input.RemoveRoleIDs != nil {
		mut.RemoveRoleIDs(input.RemoveRoleIDs...)
	}

	if input.ClearRoles {
		mut.ClearRoles()
	}

	user, err := mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Invalidate cache
	s.invalidateUserCache(ctx, id)

	return user, nil
}

// UpdateOwnProfile updates fields users are allowed to change for their own account.
func (s *UserService) UpdateOwnProfile(ctx context.Context, input ent.UpdateUserInput) (*ent.User, error) {
	currentUser, ok := contexts.GetUser(ctx)
	if !ok || currentUser == nil {
		return nil, fmt.Errorf("user not found in context")
	}

	id := currentUser.ID

	return authz.RunWithSystemBypass(ctx, "update-own-profile", func(ctx context.Context) (*ent.User, error) {
		client := s.entFromContext(ctx)

		mut := client.User.UpdateOneID(id).
			SetNillableFirstName(input.FirstName).
			SetNillableLastName(input.LastName).
			SetNillablePreferLanguage(input.PreferLanguage)

		if input.Password != nil {
			hashedPassword, err := HashPassword(*input.Password)
			if err != nil {
				return nil, err
			}

			mut.SetPassword(hashedPassword)
		}

		user, err := mut.Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to update user profile: %w", err)
		}

		// Invalidate cache
		s.invalidateUserCache(ctx, id)

		return user, nil
	})
}

// UpdateUserStatus updates the status of a user.
func (s *UserService) UpdateUserStatus(ctx context.Context, id int, status user.Status) (*ent.User, error) {
	var updatedUser *ent.User

	err := s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		result, err := client.User.UpdateOneID(id).
			SetStatus(status).
			Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to update user status: %w", err)
		}
		updatedUser = result

		if status == user.StatusDeactivated && s.apiKeyService != nil {
			if err := s.apiKeyService.disablePersonalAPIKeysByUser(ctx, id); err != nil {
				return fmt.Errorf("failed to disable user personal API keys: %w", err)
			}
		}

		runAfterCommit(ctx, func(ctx context.Context) {
			s.invalidateUserCache(ctx, id)
		})

		return nil
	})
	if err != nil {
		return nil, err
	}

	return updatedUser, nil
}

// GetUserByID gets a user by ID with caching.
func (s *UserService) GetUserByID(ctx context.Context, id int) (*ent.User, error) {
	// Try cache first
	cacheKey := buildUserCacheKey(id)
	if user, err := s.UserCache.Get(ctx, cacheKey); err == nil {
		return &user, nil
	}

	// Query database
	client := s.entFromContext(ctx)
	if client == nil {
		return nil, fmt.Errorf("ent client not found in context")
	}

	user, err := client.User.Query().
		Where(user.IDEQ(id)).
		WithRoles().
		WithProjects().
		WithProjectUsers().
		WithOidcIdentities().
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	// Cache the user
	// TODO: handle role scope changed.
	err = s.UserCache.Set(ctx, cacheKey, *user)
	if err != nil {
		log.Warn(ctx, "failed to cache user", zap.Error(err))
	}

	return user, nil
}

func buildUserCacheKey(id int) string {
	return fmt.Sprintf("user:%d", id)
}

// invalidateUserCache removes a user from cache.
func (s *UserService) invalidateUserCache(ctx context.Context, id int) {
	cacheKey := buildUserCacheKey(id)
	_ = s.UserCache.Delete(ctx, cacheKey)
}

// clearUserCache clears all user cache.
func (s *UserService) clearUserCache(ctx context.Context) {
	_ = s.UserCache.Clear(ctx)
}

// ConvertUserToUserInfo converts ent.User to objects.UserInfo.
// This method handles the conversion of user data including roles, scopes, and projects.
// Note: This function panics if the provided user is nil.
func ConvertUserToUserInfo(ctx context.Context, u *ent.User) *objects.UserInfo {
	// Convert ent.Role to objects.RoleInfo (global roles only)
	userRoles := make([]objects.RoleInfo, 0)

	for _, r := range u.Edges.Roles {
		if !r.IsSystemRole() {
			// Skip project-specific roles, they will be included in project info
			continue
		}

		userRoles = append(userRoles, objects.RoleInfo{
			Name: r.Name,
		})
	}

	// Calculate all scopes (user scopes + global role scopes)
	allScopes := make(map[string]bool)

	// Add user's direct scopes
	for _, scope := range u.Scopes {
		allScopes[scope] = true
	}

	projectRoles := map[int][]*ent.Role{}

	// Add scopes from all global roles
	for _, r := range u.Edges.Roles {
		if !r.IsSystemRole() {
			projectRoles[*r.ProjectID] = append(projectRoles[*r.ProjectID], r)
			continue
		}

		for _, scope := range r.Scopes {
			allScopes[scope] = true
		}
	}

	// Convert user projects to objects.UserProjectInfo
	userProjects := make([]objects.UserProjectInfo, 0, len(u.Edges.ProjectUsers))

	for _, up := range u.Edges.ProjectUsers {
		// Convert project roles to objects.RoleInfo
		roles := projectRoles[up.ProjectID]
		effectiveScopeSet := make(map[string]bool, len(up.Scopes))
		for _, scope := range up.Scopes {
			effectiveScopeSet[scope] = true
		}

		projectRoleInfos := make([]objects.RoleInfo, 0, len(roles))
		for _, r := range roles {
			projectRoleInfos = append(projectRoleInfos, objects.RoleInfo{
				Name: r.Name,
			})
			for _, scope := range r.Scopes {
				effectiveScopeSet[scope] = true
			}
		}

		userProjects = append(userProjects, objects.UserProjectInfo{
			ProjectID:       objects.GUID{Type: ent.TypeProject, ID: up.ProjectID},
			IsOwner:         up.IsOwner,
			Scopes:          up.Scopes,
			EffectiveScopes: lo.Keys(effectiveScopeSet),
			Roles:           projectRoleInfos,
		})
	}

	// Convert OIDC identities
	oidcIdentities := make([]objects.OIDCIdentityInfo, 0, len(u.Edges.OidcIdentities))
	for _, identity := range u.Edges.OidcIdentities {
		oidcIdentities = append(oidcIdentities, objects.OIDCIdentityInfo{
			ID:      objects.GUID{Type: ent.TypeOIDCIdentity, ID: identity.ID},
			IdpName: identity.IdpName,
			Issuer:  identity.Issuer,
			Subject: identity.Subject,
			Email:   identity.Email,
		})
	}

	return &objects.UserInfo{
		ID:             objects.GUID{Type: ent.TypeUser, ID: u.ID},
		Email:          u.Email,
		FirstName:      u.FirstName,
		LastName:       u.LastName,
		IsOwner:        u.IsOwner,
		PreferLanguage: u.PreferLanguage,
		Avatar:         lo.ToPtr(objects.UserAvatarURL(u.ID)),
		Scopes:         lo.Keys(allScopes),
		Roles:          userRoles,
		Projects:       userProjects,
		OIDCIdentities: oidcIdentities,
		HasPassword:    u.Password != OIDC_ONLY_PLACEHOLDER,
	}
}

// AddUserToProject adds a user to a project with optional owner status, scopes, and roles.
func (s *UserService) AddUserToProject(ctx context.Context, userID, projectID int, isOwner *bool, scopes []string, roleIDs []int) (*ent.UserProject, error) {
	principal, hasPrincipal := authz.GetPrincipal(ctx)
	if !hasPrincipal || !principal.IsTest() {
		if scopes != nil {
			if err := s.permissionValidator.CanGrantScopes(ctx, scopes, &projectID); err != nil {
				return nil, fmt.Errorf("permission denied: %w", err)
			}
		}
		for _, roleID := range roleIDs {
			projectRole, err := authz.RunWithSystemBypass(ctx, "project-role-assignment", func(ctx context.Context) (*ent.Role, error) {
				return s.entFromContext(ctx).Role.Query().Where(role.IDEQ(roleID), role.ProjectIDEQ(projectID)).Only(ctx)
			})
			if err != nil {
				if ent.IsNotFound(err) {
					return nil, fmt.Errorf("project role %d not found", roleID)
				}
				return nil, fmt.Errorf("failed to load project role %d: %w", roleID, err)
			}
			if err := s.permissionValidator.CanGrantRole(ctx, projectRole.Scopes, &projectID); err != nil {
				return nil, fmt.Errorf("permission denied: %w", err)
			}
		}
	}

	var userProject *ent.UserProject
	err := s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)
		mut := client.UserProject.Create().
			SetUserID(userID).
			SetProjectID(projectID)

		if isOwner != nil {
			mut.SetIsOwner(*isOwner)
		}

		if scopes != nil {
			mut.SetScopes(scopes)
		}

		created, err := mut.Save(ctx)
		if err != nil {
			return fmt.Errorf("failed to add user to project: %w", err)
		}
		userProject = created

		if len(roleIDs) == 0 {
			return nil
		}
		user, err := client.User.Get(ctx, userID)
		if err != nil {
			return fmt.Errorf("failed to get user: %w", err)
		}
		if err := user.Update().AddRoleIDs(roleIDs...).Exec(ctx); err != nil {
			return fmt.Errorf("failed to add roles to user: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	// Invalidate user cache
	s.invalidateUserCache(ctx, userID)

	return userProject, nil
}

// RemoveUserFromProject removes a user from a project.
func (s *UserService) RemoveUserFromProject(ctx context.Context, userID, projectID int) error {
	client := s.entFromContext(ctx)

	// Delete the relationship (soft delete if enabled)
	rowsAffected, err := client.UserProject.Delete().Where(
		userproject.ProjectIDEQ(projectID),
		userproject.UserIDEQ(userID),
	).Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to remove user from project: %w", err)
	}

	if rowsAffected == 0 {
		return nil
	}

	projectRoleIDs, err := client.Role.Query().
		Where(
			role.ProjectIDEQ(projectID),
			role.HasUsersWith(user.IDEQ(userID)),
		).
		IDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to query user project roles: %w", err)
	}

	if len(projectRoleIDs) > 0 {
		if err := client.User.UpdateOneID(userID).RemoveRoleIDs(projectRoleIDs...).Exec(ctx); err != nil {
			return fmt.Errorf("failed to remove user project roles: %w", err)
		}
	}

	// Invalidate user cache
	s.invalidateUserCache(ctx, userID)

	return nil
}

// UpdateProjectUser updates a user's project relationship including scopes and roles.
func (s *UserService) UpdateProjectUser(ctx context.Context, userID, projectID int, isOwner *bool, scopes []string, addRoleIDs, removeRoleIDs []int) (*ent.UserProject, error) {
	// Validate permissions before updating
	if err := s.permissionValidator.CanEditUserPermissions(ctx, userID, &projectID); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Validate scope grants if scopes are being updated
	if scopes != nil {
		if err := s.permissionValidator.CanGrantScopes(ctx, scopes, &projectID); err != nil {
			return nil, fmt.Errorf("permission denied: %w", err)
		}
	}

	// Validate role grants if roles are being added
	if len(addRoleIDs) > 0 {
		for _, roleID := range addRoleIDs {
			if err := s.permissionValidator.CanEditRole(ctx, roleID, &projectID); err != nil {
				return nil, fmt.Errorf("permission denied: %w", err)
			}
		}
	}

	client := s.entFromContext(ctx)

	// Find the UserProject relationship
	userProject, err := client.UserProject.Query().
		Where(
			userproject.UserID(userID),
			userproject.ProjectID(projectID),
		).
		Only(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to find user project relationship: %w", err)
	}

	// Update the UserProject (including isOwner, scopes, and roles)
	mut := userProject.Update()

	if isOwner != nil {
		mut.SetIsOwner(*isOwner)
	}

	if scopes != nil {
		mut.SetScopes(scopes)
	}

	userProject, err = mut.Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update project user: %w", err)
	}

	// Update roles if provided
	user, err := client.User.Get(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}

	userMut := user.Update()

	if len(addRoleIDs) > 0 {
		userMut.AddRoleIDs(addRoleIDs...)
	}

	if len(removeRoleIDs) > 0 {
		userMut.RemoveRoleIDs(removeRoleIDs...)
	}

	err = userMut.Exec(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to update user roles: %w", err)
	}

	// Invalidate user cache
	s.invalidateUserCache(ctx, userID)

	return userProject, nil
}

// DeleteUser soft deletes a user and handles all related data.
// This method performs the following operations:
// 1. Validates permissions
// 2. Checks if user is owner (cannot delete owner)
// 3. Removes user from all projects (UserProject)
// 4. Removes all user roles (UserRole)
// 5. Cleans up unpublished channels/models and pending publish requests
// 6. Archives the user's personal API keys
// 7. Soft deletes the user
// 8. Invalidates user cache.
func (s *UserService) DeleteUser(ctx context.Context, id int) error {
	// Validate permissions before deleting
	if err := s.permissionValidator.CanDeleteUser(ctx, id); err != nil {
		return fmt.Errorf("permission denied: %w", err)
	}

	return s.RunInTransaction(ctx, func(ctx context.Context) error {
		client := s.entFromContext(ctx)

		// Get user to check if it's an owner
		u, err := client.User.Get(ctx, id)
		if err != nil {
			return fmt.Errorf("failed to get user: %w", err)
		}

		// Cannot delete owner users
		if u.IsOwner {
			return fmt.Errorf("cannot delete owner user, transfer ownership first")
		}

		// 1. Delete UserProject relationships
		_, err = client.UserProject.Delete().
			Where(userproject.UserIDEQ(id)).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete user projects: %w", err)
		}

		// 2. Delete UserRole relationships
		_, err = client.UserRole.Delete().
			Where(userrole.UserIDEQ(id)).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete user roles: %w", err)
		}

		// 2.5. Clean up user's channels and models based on visibility
		userChannels, _ := client.Channel.Query().Where(channel.OwnerIDEQ(id)).All(ctx)
		for _, ch := range userChannels {
			if ch.Visibility != channel.VisibilityPublished {
				client.Channel.DeleteOneID(ch.ID).Exec(ctx)
			}
		}

		userModels, _ := client.Model.Query().Where(model.OwnerIDEQ(id)).All(ctx)
		for _, m := range userModels {
			if m.Visibility != model.VisibilityPublished {
				client.Model.DeleteOneID(m.ID).Exec(ctx)
			}
		}

		// Cancel pending publish requests
		client.PublishRequest.Delete().
			Where(publishrequest.RequesterIDEQ(id), publishrequest.StatusEQ(publishrequest.StatusPending)).
			Exec(ctx)

		// 3. Archive the user's personal API keys
		if s.apiKeyService != nil {
			if err = s.apiKeyService.archivePersonalAPIKeysByUser(ctx, id); err != nil {
				return fmt.Errorf("failed to archive user personal API keys: %w", err)
			}
		}

		// 4. Soft delete the user
		err = client.User.DeleteOneID(id).Exec(ctx)
		if err != nil {
			return fmt.Errorf("failed to delete user: %w", err)
		}

		// 5. Invalidate user cache
		s.invalidateUserCache(ctx, id)

		return nil
	})
}

// ApproveUser approves a pending user, activates their account, and sends an approval email.
func (s *UserService) ApproveUser(ctx context.Context, userID int) error {
	u, err := s.entFromContext(ctx).User.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if u.Status != user.StatusPending {
		return fmt.Errorf("user is not pending approval")
	}

	rs, err := s.systemService.RegistrationSettings(ctx)
	if err != nil {
		return fmt.Errorf("get registration settings: %w", err)
	}

	_, err = s.entFromContext(ctx).User.UpdateOneID(userID).
		SetStatus(user.StatusActivated).
		SetScopes(rs.DefaultUserScopes).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("approve user: %w", err)
	}

	// Send approval email (best effort, don't fail on error)
	if s.emailService != nil {
		baseURL, _ := contexts.GetBaseURL(ctx)
		signInURL := s.emailService.BuildURLWithBase(ctx, "/sign-in", baseURL)
		_ = s.emailService.SendApprovedEmail(ctx, u.Email, u.FirstName+" "+u.LastName, signInURL)
	}

	s.invalidateUserCache(ctx, userID)
	return nil
}

// RejectUser rejects a pending user, sends a rejection email, and deletes the user record.
func (s *UserService) RejectUser(ctx context.Context, userID int) error {
	u, err := s.entFromContext(ctx).User.Get(ctx, userID)
	if err != nil {
		return fmt.Errorf("get user: %w", err)
	}
	if u.Status != user.StatusPending {
		return fmt.Errorf("user is not pending approval")
	}

	// Send rejection email (best effort, don't fail on error)
	if s.emailService != nil {
		_ = s.emailService.SendRejectedEmail(ctx, u.Email, u.FirstName+" "+u.LastName)
	}

	err = s.entFromContext(ctx).User.DeleteOneID(userID).Exec(ctx)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}

	s.invalidateUserCache(ctx, userID)
	return nil
}

// MarkEmailVerified sets the email_verified_at timestamp for a user.
func (s *UserService) MarkEmailVerified(ctx context.Context, userID int) error {
	_, err := s.entFromContext(ctx).User.UpdateOneID(userID).
		SetEmailVerifiedAt(time.Now()).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("mark email verified: %w", err)
	}
	s.invalidateUserCache(ctx, userID)
	return nil
}

// ActivateUser activates a pending user with default scopes from registration settings.
func (s *UserService) ActivateUser(ctx context.Context, userID int) error {
	rs, err := s.systemService.RegistrationSettings(ctx)
	if err != nil {
		return err
	}
	_, err = s.entFromContext(ctx).User.UpdateOneID(userID).
		SetStatus(user.StatusActivated).
		SetScopes(rs.DefaultUserScopes).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("activate user: %w", err)
	}
	s.invalidateUserCache(ctx, userID)
	return nil
}

// GetByEmail finds a user by their email address.
func (s *UserService) GetByEmail(ctx context.Context, email string) (*ent.User, error) {
	return authz.RunWithSystemBypass(ctx, "user-email-lookup", func(bypassCtx context.Context) (*ent.User, error) {
		return s.entFromContext(bypassCtx).User.Query().
			Where(user.EmailEqualFold(normalizeEmail(email))).
			Only(bypassCtx)
	})
}

// ResetPassword updates a user's password to a new hashed value.
func (s *UserService) ResetPassword(ctx context.Context, userID int, newPassword string) error {
	hashed, err := HashPassword(newPassword)
	if err != nil {
		return err
	}
	_, err = s.entFromContext(ctx).User.UpdateOneID(userID).
		SetPassword(hashed).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("reset password: %w", err)
	}
	s.invalidateUserCache(ctx, userID)
	return nil
}

// ListOwners returns all activated users who are owners.
func (s *UserService) ListOwners(ctx context.Context) ([]*ent.User, error) {
	return s.entFromContext(ctx).User.Query().
		Where(user.IsOwner(true), user.StatusEQ(user.StatusActivated)).
		All(ctx)
}
