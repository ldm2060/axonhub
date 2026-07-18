package biz

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/authz"
	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
	"github.com/ldm2060/axonhub/internal/ent/enttest"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/internal/pkg/xcache/live"
	"github.com/ldm2060/axonhub/llm/oauth"
)

func TestCanonicalizeKimiCodeCredentialsPersistsOAuthBundle(t *testing.T) {
	models := []oauth.KimiCodeModel{{ID: "kimi-for-coding", ContextLength: 262144}}
	bundle := &oauth.OAuthCredentials{
		AccessToken:  "access",
		RefreshToken: "refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
		KimiCode:     &oauth.KimiCodeMetadata{Models: models},
	}
	raw, err := bundle.ToJSON()
	require.NoError(t, err)
	credentials := objects.ChannelCredentials{APIKey: raw, APIKeys: []string{"must-be-cleared"}}

	svc := &ChannelService{}
	require.NoError(t, svc.canonicalizeKimiCodeCredentials(channel.TypeKimiCode, &credentials))
	require.Nil(t, credentials.APIKeys)
	require.NotNil(t, credentials.OAuth)
	require.Equal(t, "access", credentials.OAuth.AccessToken)
	require.Equal(t, "refresh", credentials.OAuth.RefreshToken)
	require.Equal(t, models, credentials.OAuth.KimiCode.Models)
	parsed, err := oauth.ParseCredentialsJSON(credentials.APIKey)
	require.NoError(t, err)
	require.Equal(t, "access", parsed.AccessToken)
	require.Equal(t, "refresh", parsed.RefreshToken)
}

func TestKimiCodeRefreshPersistsCredentialsAndNotifiesChannelCache(t *testing.T) {
	db := enttest.NewEntClient(t, "sqlite3", "file:ent?mode=memory&_fk=0")
	t.Cleanup(func() { _ = db.Close() })

	ctx := authz.WithTestBypass(ent.NewContext(context.Background(), db))
	models := []oauth.KimiCodeModel{{ID: "kimi-for-coding", ContextLength: 262144}}
	created, err := db.Channel.Create().
		SetType(channel.TypeKimiCode).
		SetName("kimi-code").
		SetBaseURL("https://api.kimi.com/coding/v1").
		SetStatus(channel.StatusEnabled).
		SetSupportedModels([]string{"kimi-for-coding"}).
		SetDefaultTestModel("kimi-for-coding").
		SetCredentials(objects.ChannelCredentials{OAuth: &objects.OAuthCredentials{
			AccessToken:  "old-access",
			RefreshToken: "old-refresh",
			ExpiresAt:    time.Now().Add(-time.Hour),
			KimiCode:     &oauth.KimiCodeMetadata{Models: models},
		}}).
		Save(ctx)
	require.NoError(t, err)

	svc := NewChannelServiceForTest(db)
	t.Cleanup(svc.Stop)
	notifier := &channelSyncNotifierSpy{}
	svc.channelNotifier = notifier
	previousAsyncReloadDisabled := asyncReloadDisabled
	asyncReloadDisabled = false
	t.Cleanup(func() { asyncReloadDisabled = previousAsyncReloadDisabled })

	refreshed := &oauth.OAuthCredentials{
		AccessToken:  "new-access",
		RefreshToken: "new-refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
		KimiCode:     &oauth.KimiCodeMetadata{Models: models},
	}
	require.NoError(t, svc.refreshOAuthToken(ctx, created, refreshed))

	reloaded, err := db.Channel.Get(ctx, created.ID)
	require.NoError(t, err)
	require.Equal(t, "new-access", reloaded.Credentials.OAuth.AccessToken)
	require.Equal(t, "new-refresh", reloaded.Credentials.OAuth.RefreshToken)
	require.Equal(t, models, reloaded.Credentials.OAuth.KimiCode.Models)
	parsed, err := parseKimiCodeCredentials(reloaded.Credentials)
	require.NoError(t, err)
	require.Equal(t, "new-access", parsed.AccessToken)
	require.Equal(t, "new-refresh", parsed.RefreshToken)
	require.Equal(t, models, parsed.KimiCode.Models)
	require.Equal(t, 1, notifier.notifyCount)
	require.Equal(t, live.EventForceRefresh, notifier.events[0].Type)
}
