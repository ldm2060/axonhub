package gql

import (
	"context"
	"testing"

	"github.com/samber/lo"
	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/ent/request"
	"github.com/ldm2060/axonhub/internal/ent/user"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/internal/pkg/xcache"
	"github.com/ldm2060/axonhub/internal/server/biz"
)

func setupTestMyDashboardResolver(t *testing.T) (*queryResolver, context.Context, *ent.Client, *ent.User) {
	t.Helper()

	client := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), client))

	projectService := biz.NewProjectService(biz.ProjectServiceParams{Ent: client})
	userService := biz.NewUserService(biz.UserServiceParams{
		Ent:            client,
		ProjectService: projectService,
	})
	systemService := biz.NewSystemService(biz.SystemServiceParams{
		Ent:         client,
		CacheConfig: xcache.Config{Mode: xcache.ModeMemory},
	})

	testUser, err := userService.CreateUser(ctx, ent.CreateUserInput{
		Email:    "dashboard@example.com",
		Password: "password",
		Status:   lo.ToPtr(user.StatusActivated),
	})
	require.NoError(t, err)
	loadedUser, err := userService.GetUserByID(ctx, testUser.ID)
	require.NoError(t, err)
	require.NotNil(t, loadedUser.PrivateProjectID)

	requestCtx := ent.NewContext(context.Background(), client)
	requestCtx = contexts.WithUser(requestCtx, loadedUser)
	requestCtx = authz.NewUserContext(requestCtx, loadedUser.ID)

	resolver := &queryResolver{&Resolver{
		client:        client,
		userService:   userService,
		systemService: systemService,
	}}

	return resolver, requestCtx, client, testUser
}

func TestQueryResolver_MyDashboardCountsRequestsForPrivateProject(t *testing.T) {
	resolver, ctx, client, testUser := setupTestMyDashboardResolver(t)
	defer client.Close()

	projectID := *testUser.PrivateProjectID

	completedRequest, err := client.Request.Create().
		SetProjectID(projectID).
		SetModelID("model-a").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		Save(authz.WithTestBypass(ent.NewContext(context.Background(), client)))
	require.NoError(t, err)

	_, err = client.UsageLog.Create().
		SetRequestID(completedRequest.ID).
		SetProjectID(projectID).
		SetChannelID(1).
		SetModelID("model-a").
		Save(authz.WithTestBypass(ent.NewContext(context.Background(), client)))
	require.NoError(t, err)

	_, err = client.Request.Create().
		SetProjectID(projectID).
		SetModelID("model-a").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusFailed).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		Save(authz.WithTestBypass(ent.NewContext(context.Background(), client)))
	require.NoError(t, err)

	otherProject, err := client.Project.Create().
		SetName("other-project").
		Save(authz.WithTestBypass(ent.NewContext(context.Background(), client)))
	require.NoError(t, err)

	_, err = client.Request.Create().
		SetProjectID(otherProject.ID).
		SetModelID("model-a").
		SetFormat("openai/chat_completions").
		SetStatus(request.StatusCompleted).
		SetRequestBody(objects.JSONRawMessage([]byte(`{}`))).
		Save(authz.WithTestBypass(ent.NewContext(context.Background(), client)))
	require.NoError(t, err)

	stats, err := resolver.MyDashboard(ctx)
	require.NoError(t, err)
	require.Equal(t, 2, stats.TotalRequests)
	require.Equal(t, 1, stats.FailedRequests)
}
