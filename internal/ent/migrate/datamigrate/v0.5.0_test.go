package datamigrate_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/ent/migrate/datamigrate"
	"github.com/ldm2060/axonhub/internal/ent/model"
	"github.com/ldm2060/axonhub/internal/ent/project"
	"github.com/ldm2060/axonhub/internal/ent/role"
	"github.com/ldm2060/axonhub/internal/objects"
)

func newV0_5_0TestContext(t *testing.T) (*ent.Client, context.Context) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=1")
	t.Cleanup(func() { client.Close() })

	ctx := context.Background()
	ctx = ent.NewContext(ctx, client)
	ctx = authz.WithTestBypass(ctx)

	return client, ctx
}

func createV0_5_0TestChannel(t *testing.T, client *ent.Client, ctx context.Context, name string, ownerID int) *ent.Channel {
	t.Helper()

	creator := client.Channel.Create().
		SetType(channel.TypeOpenai).
		SetName(name).
		SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
		SetSupportedModels([]string{"model-1"}).
		SetDefaultTestModel("model-1")

	if ownerID != 0 {
		creator = creator.SetOwnerID(ownerID)
	}

	ch, err := creator.Save(ctx)
	require.NoError(t, err)

	return ch
}

func createV0_5_0TestModel(t *testing.T, client *ent.Client, ctx context.Context, modelID string, ownerID int) *ent.Model {
	t.Helper()

	creator := client.Model.Create().
		SetDeveloper("test-developer").
		SetModelID(modelID).
		SetType(model.TypeChat).
		SetName(fmt.Sprintf("Test Model %s", modelID)).
		SetIcon("test-icon").
		SetGroup("test-group").
		SetModelCard(&objects.ModelCard{}).
		SetSettings(&objects.ModelSettings{}).
		SetVisibility(model.VisibilityPrivate)

	if ownerID != 0 {
		creator = creator.SetOwnerID(ownerID)
	}

	m, err := creator.Save(ctx)
	require.NoError(t, err)

	return m
}

func createV0_5_0TestUser(t *testing.T, client *ent.Client, ctx context.Context, email string) *ent.User {
	t.Helper()

	u, err := client.User.Create().
		SetEmail(email).
		SetPassword("hashedpassword").
		SetFirstName("Test").
		SetLastName("User").
		SetScopes([]string{}).
		Save(ctx)
	require.NoError(t, err)

	return u
}

func TestV0_5_0_PublishOwnerlessChannels(t *testing.T) {
	client, ctx := newV0_5_0TestContext(t)

	ownerlessCh := createV0_5_0TestChannel(t, client, ctx, "ownerless-channel", 0)

	err := datamigrate.NewV0_5_0().Migrate(ctx, client)
	require.NoError(t, err)

	got, err := client.Channel.Get(ctx, ownerlessCh.ID)
	require.NoError(t, err)
	assert.Equal(t, channel.VisibilityPublished, got.Visibility)
}

func TestV0_5_0_DoesNotPublishOwnedChannels(t *testing.T) {
	client, ctx := newV0_5_0TestContext(t)

	owner := createV0_5_0TestUser(t, client, ctx, "owner@example.com")
	ownedCh := createV0_5_0TestChannel(t, client, ctx, "owned-channel", owner.ID)

	err := datamigrate.NewV0_5_0().Migrate(ctx, client)
	require.NoError(t, err)

	got, err := client.Channel.Get(ctx, ownedCh.ID)
	require.NoError(t, err)
	assert.Equal(t, channel.VisibilityPrivate, got.Visibility)
}

func TestV0_5_0_PublishOwnerlessModels(t *testing.T) {
	client, ctx := newV0_5_0TestContext(t)

	ownerlessModel := createV0_5_0TestModel(t, client, ctx, "ownerless-model", 0)

	err := datamigrate.NewV0_5_0().Migrate(ctx, client)
	require.NoError(t, err)

	got, err := client.Model.Get(ctx, ownerlessModel.ID)
	require.NoError(t, err)
	assert.Equal(t, model.VisibilityPublished, got.Visibility)
}

func TestV0_5_0_DoesNotPublishOwnedModels(t *testing.T) {
	client, ctx := newV0_5_0TestContext(t)

	owner := createV0_5_0TestUser(t, client, ctx, "owner@example.com")
	ownedModel := createV0_5_0TestModel(t, client, ctx, "owned-model", owner.ID)

	err := datamigrate.NewV0_5_0().Migrate(ctx, client)
	require.NoError(t, err)

	got, err := client.Model.Get(ctx, ownedModel.ID)
	require.NoError(t, err)
	assert.Equal(t, model.VisibilityPrivate, got.Visibility)
}

func TestV0_5_0_CreatePrivateProjectForUser(t *testing.T) {
	client, ctx := newV0_5_0TestContext(t)

	user := createV0_5_0TestUser(t, client, ctx, "user@example.com")

	err := datamigrate.NewV0_5_0().Migrate(ctx, client)
	require.NoError(t, err)

	// The user's private_project_id should now point at the newly created project.
	got, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, got.PrivateProjectID)
	require.NotZero(t, *got.PrivateProjectID)

	proj, err := client.Project.Get(ctx, *got.PrivateProjectID)
	require.NoError(t, err)
	expectedName := fmt.Sprintf("__user_%d__", user.ID)
	assert.Equal(t, expectedName, proj.Name)

	// The user should be the owner of that project via user_project.
	userProjects, err := client.UserProject.Query().All(ctx)
	require.NoError(t, err)
	require.Len(t, userProjects, 1)
	assert.Equal(t, user.ID, userProjects[0].UserID)
	assert.Equal(t, proj.ID, userProjects[0].ProjectID)
	assert.True(t, userProjects[0].IsOwner)

	// Default project roles (Admin / Developer / Viewer) should be created for the project.
	roles, err := client.Role.Query().Where(role.ProjectIDEQ(proj.ID)).All(ctx)
	require.NoError(t, err)
	assert.Len(t, roles, 3)

	roleNames := make(map[string]bool)
	for _, r := range roles {
		roleNames[r.Name] = true
		assert.Equal(t, role.LevelProject, r.Level)
	}
	assert.True(t, roleNames["Admin"])
	assert.True(t, roleNames["Developer"])
	assert.True(t, roleNames["Viewer"])
}

func TestV0_5_0_SkipUserWithExistingPrivateProject(t *testing.T) {
	client, ctx := newV0_5_0TestContext(t)

	// Pre-create a project and link it as the user's private project.
	existingProj, err := client.Project.Create().
		SetName("existing-private").
		SetDescription("existing").
		Save(ctx)
	require.NoError(t, err)

	user, err := client.User.Create().
		SetEmail("user@example.com").
		SetPassword("hashedpassword").
		SetFirstName("Test").
		SetLastName("User").
		SetScopes([]string{}).
		SetPrivateProjectID(existingProj.ID).
		Save(ctx)
	require.NoError(t, err)

	err = datamigrate.NewV0_5_0().Migrate(ctx, client)
	require.NoError(t, err)

	got, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, got.PrivateProjectID)
	assert.Equal(t, existingProj.ID, *got.PrivateProjectID, "private_project_id should not be changed")

	// No new projects should be created for this user.
	projectCount, err := client.Project.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, projectCount)

	// No user_project rows should be created (the migration only creates them for new projects).
	userProjectCount, err := client.UserProject.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, userProjectCount)
}

func TestV0_5_0_Idempotency(t *testing.T) {
	client, ctx := newV0_5_0TestContext(t)

	ownerlessCh := createV0_5_0TestChannel(t, client, ctx, "ownerless-channel", 0)
	ownerlessModel := createV0_5_0TestModel(t, client, ctx, "ownerless-model", 0)
	user := createV0_5_0TestUser(t, client, ctx, "user@example.com")

	// First run.
	err := datamigrate.NewV0_5_0().Migrate(ctx, client)
	require.NoError(t, err)

	userAfter1, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, userAfter1.PrivateProjectID)
	privateProjectIDAfter1 := *userAfter1.PrivateProjectID

	projectCountAfter1, err := client.Project.Query().Count(ctx)
	require.NoError(t, err)

	roleCountAfter1, err := client.Role.Query().Count(ctx)
	require.NoError(t, err)

	userProjectCountAfter1, err := client.UserProject.Query().Count(ctx)
	require.NoError(t, err)

	// Second run — should be a no-op for everything we touched.
	err = datamigrate.NewV0_5_0().Migrate(ctx, client)
	require.NoError(t, err)

	chAfter2, err := client.Channel.Get(ctx, ownerlessCh.ID)
	require.NoError(t, err)
	assert.Equal(t, channel.VisibilityPublished, chAfter2.Visibility)

	modelAfter2, err := client.Model.Get(ctx, ownerlessModel.ID)
	require.NoError(t, err)
	assert.Equal(t, model.VisibilityPublished, modelAfter2.Visibility)

	userAfter2, err := client.User.Get(ctx, user.ID)
	require.NoError(t, err)
	require.NotNil(t, userAfter2.PrivateProjectID)
	assert.Equal(t, privateProjectIDAfter1, *userAfter2.PrivateProjectID, "private_project_id should remain stable across runs")

	projectCountAfter2, err := client.Project.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, projectCountAfter1, projectCountAfter2, "no new projects on second run")

	roleCountAfter2, err := client.Role.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, roleCountAfter1, roleCountAfter2, "no new roles on second run")

	userProjectCountAfter2, err := client.UserProject.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, userProjectCountAfter1, userProjectCountAfter2, "no new user_project rows on second run")
}

func TestV0_5_0_MultipleUsers(t *testing.T) {
	client, ctx := newV0_5_0TestContext(t)

	user1 := createV0_5_0TestUser(t, client, ctx, "user1@example.com")
	user2 := createV0_5_0TestUser(t, client, ctx, "user2@example.com")
	user3 := createV0_5_0TestUser(t, client, ctx, "user3@example.com")

	err := datamigrate.NewV0_5_0().Migrate(ctx, client)
	require.NoError(t, err)

	for _, u := range []*ent.User{user1, user2, user3} {
		got, err := client.User.Get(ctx, u.ID)
		require.NoError(t, err)
		require.NotNil(t, got.PrivateProjectID, "user %d should have a private project", u.ID)

		proj, err := client.Project.Query().
			Where(project.NameEQ(fmt.Sprintf("__user_%d__", u.ID))).
			Only(ctx)
		require.NoError(t, err)
		assert.Equal(t, *got.PrivateProjectID, proj.ID)
	}

	// Each user should have one user_project row marking them as owner of their private project.
	userProjectCount, err := client.UserProject.Query().Count(ctx)
	require.NoError(t, err)
	assert.Equal(t, 3, userProjectCount)
}

func TestV0_5_0_MixedOwnedAndOwnerlessResources(t *testing.T) {
	client, ctx := newV0_5_0TestContext(t)

	owner := createV0_5_0TestUser(t, client, ctx, "owner@example.com")

	ownerlessCh := createV0_5_0TestChannel(t, client, ctx, "ownerless-channel", 0)
	ownedCh := createV0_5_0TestChannel(t, client, ctx, "owned-channel", owner.ID)
	ownerlessModel := createV0_5_0TestModel(t, client, ctx, "ownerless-model", 0)
	ownedModel := createV0_5_0TestModel(t, client, ctx, "owned-model", owner.ID)

	err := datamigrate.NewV0_5_0().Migrate(ctx, client)
	require.NoError(t, err)

	gotOwnerlessCh, err := client.Channel.Get(ctx, ownerlessCh.ID)
	require.NoError(t, err)
	assert.Equal(t, channel.VisibilityPublished, gotOwnerlessCh.Visibility)

	gotOwnedCh, err := client.Channel.Get(ctx, ownedCh.ID)
	require.NoError(t, err)
	assert.Equal(t, channel.VisibilityPrivate, gotOwnedCh.Visibility)

	gotOwnerlessModel, err := client.Model.Get(ctx, ownerlessModel.ID)
	require.NoError(t, err)
	assert.Equal(t, model.VisibilityPublished, gotOwnerlessModel.Visibility)

	gotOwnedModel, err := client.Model.Get(ctx, ownedModel.ID)
	require.NoError(t, err)
	assert.Equal(t, model.VisibilityPrivate, gotOwnedModel.Visibility)
}
