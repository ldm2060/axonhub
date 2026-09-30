package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/ldm2060/axonhub/internal/ent/schema/schematype"
	"github.com/ldm2060/axonhub/internal/scopes"
)

// OAuthClient holds the schema definition for third-party applications that
// authenticate users through AxonHub acting as an OIDC provider.
type OAuthClient struct {
	ent.Schema
}

func (OAuthClient) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

// Fields of the OAuthClient.
func (OAuthClient) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").
			Comment("OAuth client display name"),
		field.String("description").
			Optional().
			Default("").
			Comment("OAuth client description"),
		field.String("client_id").
			Immutable().
			Comment("OAuth2 client identifier"),
		field.String("client_secret_hash").
			Sensitive().
			Annotations(entgql.Skip(entgql.SkipAll)).
			Comment("SHA-256 hash of the OAuth2 client secret"),
		field.Strings("redirect_uris").
			Comment("Allowed OAuth2 redirect URIs (exact match)"),
		field.Enum("client_type").
			Values("confidential", "public").
			Default("confidential").
			Comment("confidential clients hold a secret, public clients rely on PKCE only"),
		field.Enum("status").
			Values("enabled", "disabled").
			Default("enabled").
			Comment("OAuth client status"),
		field.Time("last_used_at").
			Optional().
			Nillable().
			Annotations(entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput)).
			Comment("Last time this client exchanged an authorization code"),
		field.Int("user_id").
			Immutable().
			Comment("Reference to the user that created the client"),
	}
}

// Edges of the OAuthClient.
func (OAuthClient) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("oauth_clients").
			Field("user_id").
			Unique().
			Required().
			Immutable(),
	}
}

func (OAuthClient) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("client_id").
			StorageKey("oauth_clients_by_client_id").
			Unique(),
		index.Fields("name", "deleted_at").
			StorageKey("oauth_clients_by_name_deleted_at").
			Unique(),
		index.Fields("user_id").
			StorageKey("oauth_clients_by_user_id"),
	}
}

func (OAuthClient) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField("oauthClients"),
		entgql.RelayConnection(),
	}
}

func (OAuthClient) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.OwnerRule(),
			scopes.UserReadScopeRule(scopes.ScopeReadSettings),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWriteSettings),
		},
	}
}
