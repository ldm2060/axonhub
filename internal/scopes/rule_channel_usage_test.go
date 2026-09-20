package scopes

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ldm2060/axonhub/internal/ent"
	"github.com/ldm2060/axonhub/internal/ent/channel"
)

func TestCanRouteThroughChannel(t *testing.T) {
	const (
		ownerID = 1
		otherID = 2
	)

	tests := []struct {
		name string
		user *ent.User
		ch   *ent.Channel
		want bool
	}{
		{
			name: "no user principal is a system-level caller",
			user: nil,
			ch:   &ent.Channel{OwnerID: ownerID, Visibility: channel.VisibilityPrivate},
			want: true,
		},
		{
			name: "system owner reaches every channel",
			user: &ent.User{ID: otherID, IsOwner: true},
			ch:   &ent.Channel{OwnerID: ownerID, Visibility: channel.VisibilityPrivate},
			want: true,
		},
		{
			name: "write_channels holder reaches every channel",
			user: &ent.User{ID: otherID, Scopes: []string{string(ScopeWriteChannels)}},
			ch:   &ent.Channel{OwnerID: ownerID, Visibility: channel.VisibilityPrivate},
			want: true,
		},
		{
			name: "channel owner reaches their own private channel",
			user: &ent.User{ID: ownerID, Scopes: []string{string(ScopeManageOwnChannels)}},
			ch:   &ent.Channel{OwnerID: ownerID, Visibility: channel.VisibilityPrivate},
			want: true,
		},
		{
			name: "another user cannot reach a private channel",
			user: &ent.User{ID: otherID, Scopes: []string{string(ScopeManageOwnChannels)}},
			ch:   &ent.Channel{OwnerID: ownerID, Visibility: channel.VisibilityPrivate},
			want: false,
		},
		{
			name: "another user cannot reach an unshared shared channel",
			user: &ent.User{ID: otherID},
			ch:   &ent.Channel{OwnerID: ownerID, Visibility: channel.VisibilityShared, SharedWith: []int{3}},
			want: false,
		},
		{
			name: "shared_with member reaches the shared channel",
			user: &ent.User{ID: otherID},
			ch:   &ent.Channel{OwnerID: ownerID, Visibility: channel.VisibilityShared, SharedWith: []int{otherID}},
			want: true,
		},
		{
			name: "published channel is open to everyone",
			user: &ent.User{ID: otherID},
			ch:   &ent.Channel{OwnerID: ownerID, Visibility: channel.VisibilityPublished},
			want: true,
		},
		{
			name: "unowned channel stays open",
			user: &ent.User{ID: otherID},
			ch:   &ent.Channel{Visibility: channel.VisibilityPrivate},
			want: true,
		},
		{
			name: "read_channels alone does not grant access to private channels",
			user: &ent.User{ID: otherID, Scopes: []string{string(ScopeReadChannels)}},
			ch:   &ent.Channel{OwnerID: ownerID, Visibility: channel.VisibilityPrivate},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, CanRouteThroughChannel(tt.user, tt.ch))
		})
	}
}
