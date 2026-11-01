package orchestrator

import (
	"context"
	"strconv"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/internal/scopes"
	"github.com/ldm2060/axonhub/llm"
)

// stubPoolingGate answers from a "userID:channelID" map; anything not listed is
// allowed, so a test only declares the pairs that must be dropped.
type stubPoolingGate struct {
	answers map[string]bool
}

func (g stubPoolingGate) AllowRoutingThroughChannel(_ context.Context, user *ent.User, ch *ent.Channel) bool {
	if user == nil || ch == nil {
		return true
	}
	key := strconv.Itoa(user.ID) + ":" + strconv.Itoa(ch.ID)
	if allowed, ok := g.answers[key]; ok {
		return allowed
	}
	return true
}

func TestDefaultSelector_Select_DropsExhaustedPoolMembers(t *testing.T) {
	ctx, client := setupTest(t)

	const (
		ownerID  = 101
		memberID = 202
	)

	create := func(name string, chOwnerID int, visibility channel.Visibility, sharedWith []int) *ent.Channel {
		builder := client.Channel.Create().
			SetType(channel.TypeOpenai).
			SetName(name).
			SetBaseURL("https://api.openai.com/v1").
			SetCredentials(objects.ChannelCredentials{APIKey: "test-key"}).
			SetSupportedModels([]string{"gpt-4"}).
			SetDefaultTestModel("gpt-4").
			SetStatus(channel.StatusEnabled).
			SetVisibility(visibility)
		if chOwnerID != 0 {
			builder = builder.SetOwnerID(chOwnerID)
		}
		if len(sharedWith) > 0 {
			builder = builder.SetSharedWith(sharedWith)
		}
		ch, err := builder.Save(ctx)
		require.NoError(t, err)
		return ch
	}

	pooled := create("Pooled Channel", ownerID, channel.VisibilityShared, []int{memberID})
	plain := create("Plain Channel", ownerID, channel.VisibilityShared, []int{memberID})

	channelService := newTestChannelServiceForChannels(client)
	modelService := newTestModelService(client)
	systemService := newTestSystemService(client)

	// The member has exhausted their share on the pooled channel only.
	gate := stubPoolingGate{answers: map[string]bool{
		strconv.Itoa(memberID) + ":" + strconv.Itoa(pooled.ID): false,
	}}
	selector := NewDefaultSelector(channelService, modelService, systemService, gate)
	req := &llm.Request{Model: "gpt-4"}

	t.Run("exhausted member loses the pooled channel but keeps the rest", func(t *testing.T) {
		userCtx := contexts.WithPrincipalUser(ctx, &ent.User{
			ID:     memberID,
			Scopes: []string{string(scopes.ScopeReadChannels)},
		})

		result, err := selector.Select(userCtx, req)
		require.NoError(t, err)

		ids := channelIDsOf(result)
		require.Contains(t, ids, plain.ID)
		require.NotContains(t, ids, pooled.ID)
	})

	t.Run("a user outside the pool keeps every channel", func(t *testing.T) {
		otherCtx := contexts.WithPrincipalUser(ctx, &ent.User{
			ID:      ownerID,
			IsOwner: true,
		})

		result, err := selector.Select(otherCtx, req)
		require.NoError(t, err)

		ids := channelIDsOf(result)
		require.Contains(t, ids, pooled.ID)
		require.Contains(t, ids, plain.ID)
	})

	t.Run("system path ignores the gate entirely", func(t *testing.T) {
		result, err := selector.Select(ctx, req)
		require.NoError(t, err)

		ids := channelIDsOf(result)
		require.Contains(t, ids, pooled.ID)
		require.Contains(t, ids, plain.ID)
	})
}
