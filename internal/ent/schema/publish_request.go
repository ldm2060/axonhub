package schema

import (
	"context"

	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/entql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/ldm2060/axonhub/internal/contexts"
	"github.com/ldm2060/axonhub/internal/ent/privacy"
	"github.com/ldm2060/axonhub/internal/ent/schema/schematype"
	"github.com/ldm2060/axonhub/internal/scopes"
)

type PublishRequest struct {
	ent.Schema
}

func (PublishRequest) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (PublishRequest) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (PublishRequest) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("resource_type").Values("channel", "model").Immutable(),
		field.Int("resource_id").Immutable(),
		field.Int("requester_id").Immutable().Annotations(entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput)),
		field.Enum("status").Values("pending", "approved", "rejected").Default("pending").Annotations(entgql.Skip(entgql.SkipMutationCreateInput)),
		field.Int("reviewer_id").Optional().Nillable().Annotations(entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput)),
		field.String("review_comment").Optional().Nillable(),
		field.String("request_comment").Optional().Nillable(),
	}
}

func (PublishRequest) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("requester", User.Type).Ref("publish_requests").Unique().Required().Immutable().Field("requester_id").Annotations(entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput)),
		edge.From("reviewer", User.Type).Ref("reviewed_requests").Unique().Field("reviewer_id").Annotations(entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput)),
	}
}

func (PublishRequest) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("status", "deleted_at").StorageKey("publish_requests_by_status"),
		index.Fields("requester_id", "deleted_at").StorageKey("publish_requests_by_requester"),
	}
}

func (PublishRequest) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.OwnerRule(),
			scopes.UserReadScopeRule(scopes.ScopeReviewPublishRequests),
			requesterQueryRule(),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeReviewPublishRequests),
		},
	}
}

// requesterQueryRule allows users to query publish requests where they are the requester.
func requesterQueryRule() privacy.QueryRule {
	return privacy.FilterFunc(func(ctx context.Context, f privacy.Filter) error {
		user, ok := contexts.GetUser(ctx)
		if !ok || user == nil {
			return privacy.Skipf("no user in context")
		}

		f.Where(entql.FieldEQ("requester_id", user.ID))

		return privacy.Allowf("User %d can query own publish requests", user.ID)
	})
}
