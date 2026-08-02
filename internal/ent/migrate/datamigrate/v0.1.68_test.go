package datamigrate_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/ent/migrate/datamigrate"
	"github.com/ldm2060/axonhub/internal/ent/role"
)

func TestV0_1_68_UpdatesUnchangedDeveloperPreset(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:developer-scope-update?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	project := client.Project.Create().SetName("developer-scope-update-project").SaveX(ctx)
	developerRole := client.Role.Create().
		SetName("Developer").
		SetLevel(role.LevelProject).
		SetProjectID(project.ID).
		SetScopes([]string{"read_api_keys", "write_api_keys", "read_prompts", "write_prompts", "write_requests"}).
		SaveX(ctx)

	require.NoError(t, datamigrate.NewV0_1_68().Migrate(ctx, client))
	updated := client.Role.GetX(ctx, developerRole.ID)
	require.ElementsMatch(t, []string{"read_api_keys", "write_api_keys", "read_requests", "write_requests"}, updated.Scopes)
}

func TestV0_1_68_PreservesCustomizedDeveloperRole(t *testing.T) {
	client := enttest.NewEntClient(t, "sqlite3", "file:developer-scope-custom?mode=memory&_fk=1")
	defer client.Close()

	ctx := authz.WithTestBypass(context.Background())
	project := client.Project.Create().SetName("developer-scope-custom-project").SaveX(ctx)
	developerRole := client.Role.Create().
		SetName("Developer").
		SetLevel(role.LevelProject).
		SetProjectID(project.ID).
		SetScopes([]string{"read_api_keys", "write_api_keys", "write_requests", "custom_scope"}).
		SaveX(ctx)

	require.NoError(t, datamigrate.NewV0_1_68().Migrate(ctx, client))
	updated := client.Role.GetX(ctx, developerRole.ID)
	require.ElementsMatch(t, []string{"read_api_keys", "write_api_keys", "write_requests", "custom_scope"}, updated.Scopes)
}
