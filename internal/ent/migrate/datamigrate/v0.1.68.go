package datamigrate

import (
	"context"
	"fmt"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/role"
	"github.com/ldm2060/axonhub/internal/scopes"
)

// V0_1_68 updates the default project Developer role scopes.
type V0_1_68 struct{}

// NewV0_1_68 creates the v0.1.68 data migrator.
func NewV0_1_68() DataMigrator {
	return &V0_1_68{}
}

// Version returns the migration version.
func (v *V0_1_68) Version() string {
	return "v0.1.68"
}

// Migrate removes prompt permissions and adds request read access to unchanged Developer presets.
func (v *V0_1_68) Migrate(ctx context.Context, client *ent.Client) (err error) {
	ctx = authz.WithSystemBypass(ctx, "database-migrate")
	ctx, tx, err := client.OpenTx(ctx)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	txClient := ent.FromContext(ctx)
	roles, err := txClient.Role.Query().Where(
		role.LevelEQ(role.LevelProject),
		role.NameEQ("Developer"),
	).All(ctx)
	if err != nil {
		return fmt.Errorf("query project Developer roles: %w", err)
	}

	legacyScopes := []string{
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
		string(scopes.ScopeReadPrompts),
		string(scopes.ScopeWritePrompts),
		string(scopes.ScopeWriteRequests),
	}
	defaultScopes := []string{
		string(scopes.ScopeReadAPIKeys),
		string(scopes.ScopeWriteAPIKeys),
		string(scopes.ScopeReadRequests),
		string(scopes.ScopeWriteRequests),
	}

	for _, developerRole := range roles {
		if !sameScopes(developerRole.Scopes, legacyScopes) {
			continue
		}

		if err := txClient.Role.UpdateOneID(developerRole.ID).SetScopes(defaultScopes).Exec(ctx); err != nil {
			return fmt.Errorf("update Developer role %d scopes: %w", developerRole.ID, err)
		}
	}

	return tx.Commit()
}
