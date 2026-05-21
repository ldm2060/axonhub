package datamigrate

import (
	"context"
	"fmt"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/model"
	"github.com/ldm2060/axonhub/internal/ent/role"
	"github.com/ldm2060/axonhub/internal/log"
	"github.com/ldm2060/axonhub/internal/scopes"
)

type V0_5_0 struct{}

func NewV0_5_0() DataMigrator {
	return &V0_5_0{}
}

func (v *V0_5_0) Version() string {
	return "v0.5.0"
}

func (v *V0_5_0) Migrate(ctx context.Context, client *ent.Client) error {
	ctx = authz.WithSystemBypass(context.Background(), "database-migrate")

	// Step 1: Set all existing channels without an owner as published
	channelUpdated, err := client.Channel.Update().
		Where(channel.OwnerIDIsNil()).
		SetVisibility(channel.VisibilityPublished).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("migrate channels visibility: %w", err)
	}
	if channelUpdated > 0 {
		log.Info(ctx, "set existing channels as published", log.Int("count", channelUpdated))
	}

	// Step 2: Set all existing models without an owner as published
	modelUpdated, err := client.Model.Update().
		Where(model.OwnerIDIsNil()).
		SetVisibility(model.VisibilityPublished).
		Save(ctx)
	if err != nil {
		return fmt.Errorf("migrate models visibility: %w", err)
	}
	if modelUpdated > 0 {
		log.Info(ctx, "set existing models as published", log.Int("count", modelUpdated))
	}

	// Step 3: For each user without a private project, create one
	users, err := client.User.Query().All(ctx)
	if err != nil {
		return fmt.Errorf("query users: %w", err)
	}

	for _, u := range users {
		if u.PrivateProjectID != nil && *u.PrivateProjectID != 0 {
			continue
		}

		projectName := fmt.Sprintf("__user_%d__", u.ID)
		proj, err := client.Project.Create().
			SetName(projectName).
			SetDescription("Private project for user " + fmt.Sprint(u.ID)).
			Save(ctx)
		if err != nil {
			log.Warn(ctx, "failed to create private project for user", log.Int("user_id", u.ID))
			continue
		}

		_, err = client.UserProject.Create().
			SetUserID(u.ID).
			SetProjectID(proj.ID).
			SetIsOwner(true).
			SetScopes([]string{}).
			Save(ctx)
		if err != nil {
			log.Warn(ctx, "failed to assign project owner", log.Int("user_id", u.ID))
			continue
		}

		err = client.User.UpdateOneID(u.ID).
			SetPrivateProjectID(proj.ID).
			Exec(ctx)
		if err != nil {
			log.Warn(ctx, "failed to set private_project_id", log.Int("user_id", u.ID))
			continue
		}

		createDefaultProjectRoles(ctx, client, proj.ID)
		log.Info(ctx, "created private project for user", log.Int("user_id", u.ID), log.Int("project_id", proj.ID))
	}

	return nil
}

func createDefaultProjectRoles(ctx context.Context, client *ent.Client, projectID int) {
	roles := []struct {
		name   string
		scopes []string
	}{
		{"Admin", []string{
			string(scopes.ScopeReadUsers), string(scopes.ScopeWriteUsers),
			string(scopes.ScopeReadRoles), string(scopes.ScopeWriteRoles),
			string(scopes.ScopeReadAPIKeys), string(scopes.ScopeWriteAPIKeys),
			string(scopes.ScopeReadRequests), string(scopes.ScopeWriteRequests),
		}},
		{"Developer", []string{
			string(scopes.ScopeReadUsers),
			string(scopes.ScopeReadAPIKeys), string(scopes.ScopeWriteAPIKeys),
			string(scopes.ScopeReadRequests),
		}},
		{"Viewer", []string{
			string(scopes.ScopeReadUsers),
			string(scopes.ScopeReadRequests),
		}},
	}
	for _, r := range roles {
		client.Role.Create().
			SetName(r.name).
			SetLevel(role.LevelProject).
			SetProjectID(projectID).
			SetScopes(r.scopes).
			SaveX(ctx)
	}
}
