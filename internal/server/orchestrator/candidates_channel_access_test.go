package orchestrator

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/internal/scopes"
	"github.com/ldm2060/axonhub/llm"
)

func TestDefaultSelector_Select_DropsChannelsTheActingUserCannotRouteThrough(t *testing.T) {
	ctx, client := setupTest(t)

	const (
		ownerID  = 101
		viewerID = 202
		otherID  = 303
	)

	create := func(name string, ownerID int, visibility channel.Visibility, sharedWith []int) *ent.Channel {
		builder := client.Channel.Create().
			SetType(channel.TypeOpenai).
			SetName(name).
			SetBaseURL("https://api.openai.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetStatus(channel.StatusEnabled).
			SetVisibility(visibility)

		if ownerID != 0 {
			builder = builder.SetOwnerID(ownerID)
		}

		if len(sharedWith) > 0 {
			builder = builder.SetSharedWith(sharedWith)
		}

		ch, err := builder.Save(ctx)
		require.NoError(t, err)

		return ch
	}

	ownPrivate := create("Own Private", viewerID, channel.VisibilityPrivate, nil)
	otherPrivate := create("Other Private", ownerID, channel.VisibilityPrivate, nil)
	sharedWithViewer := create("Shared With Viewer", ownerID, channel.VisibilityShared, []int{viewerID})
	sharedWithOther := create("Shared With Someone Else", ownerID, channel.VisibilityShared, []int{otherID})
	published := create("Published", ownerID, channel.VisibilityPublished, nil)
	unowned := create("Unowned", 0, channel.VisibilityPrivate, nil)

	channelService := newTestChannelServiceForChannels(client)
	modelService := newTestModelService(client)
	systemService := newTestSystemService(client)

	selector := NewDefaultSelector(channelService, modelService, systemService)
	req := &llm.Request{Model: "gpt-4"}

	t.Run("acting user only reaches own, shared and published channels", func(t *testing.T) {
		userCtx := contexts.WithPrincipalUser(ctx, &ent.User{
			ID:     viewerID,
			Scopes: []string{string(scopes.ScopeManageOwnChannels), string(scopes.ScopeReadChannels)},
		})

		result, err := selector.Select(userCtx, req)
		require.NoError(t, err)

		ids := channelIDsOf(result)
		require.Contains(t, ids, ownPrivate.ID)
		require.Contains(t, ids, sharedWithViewer.ID)
		require.Contains(t, ids, published.ID)
		require.Contains(t, ids, unowned.ID)
		require.NotContains(t, ids, otherPrivate.ID)
		require.NotContains(t, ids, sharedWithOther.ID)
	})

	t.Run("system owner reaches every channel", func(t *testing.T) {
		ownerCtx := contexts.WithPrincipalUser(ctx, &ent.User{ID: ownerID, IsOwner: true})

		result, err := selector.Select(ownerCtx, req)
		require.NoError(t, err)
		require.Len(t, channelIDsOf(result), 6)
	})

	t.Run("a request without an acting user keeps the system path", func(t *testing.T) {
		result, err := selector.Select(ctx, req)
		require.NoError(t, err)
		require.Len(t, channelIDsOf(result), 6)
	})
}

func channelIDsOf(candidates []*ChannelModelsCandidate) []int {
	ids := make([]int, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate != nil && candidate.Channel != nil {
			ids = append(ids, candidate.Channel.ID)
		}
	}

	return ids
}
