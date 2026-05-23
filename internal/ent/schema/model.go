package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/ldm2060/axonhub/internal/ent/schema/schematype"
	"github.com/ldm2060/axonhub/internal/objects"
	"github.com/ldm2060/axonhub/internal/scopes"
)

type Model struct {
	ent.Schema
}

func (Model) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (Model) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name", "deleted_at").
			StorageKey("models_by_name").
			Unique(),
		index.Fields("model_id", "deleted_at").
			StorageKey("models_by_model_id").
			Unique(),
		index.Fields("owner_id", "deleted_at").StorageKey("models_by_owner_id"),
	}
}

func (Model) Fields() []ent.Field {
	return []ent.Field{
		field.String("developer").Comment("developer of the model, eg. deeepseek"),
		field.String("model_id").Comment("model id, eg. deeepseek-chat"),
		field.Enum("type").Values("chat", "embedding", "rerank", "image_generation", "video_generation").Default("chat").Comment("model type"),
		field.String("name").Comment("model name, eg. DeepSeek Chat").
			Annotations(
				entgql.OrderField("NAME"),
			),
		field.String("icon").Comment("icon of the model from the lobe-icons, eg. DeepSeek"),
		field.String("group").Comment("model group, eg. deepseek"),
		field.JSON("model_card", &objects.ModelCard{}),
		field.JSON("settings", &objects.ModelSettings{}),
		field.Enum("status").Values("enabled", "disabled", "archived").Default("disabled").Annotations(
			entgql.Skip(entgql.SkipMutationCreateInput),
		),
		field.String("remark").
			Optional().Nillable().
			Comment("User-defined remark or note for the Model"),
			field.Int("owner_id").Optional().Immutable().Annotations(entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput)),
			field.Enum("visibility").Values("private", "shared", "published").Default("published").Annotations(entgql.OrderField("VISIBILITY")),
			field.JSON("shared_with", []int{}).Optional().Annotations(entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput)),
	}
}

func (Model) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("owner", User.Type).Ref("owned_models").Unique().Immutable().Field("owner_id").Annotations(entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput)),
	}
}

func (Model) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

func (Model) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.APIKeyScopeQueryRule(scopes.ScopeReadChannels),
			scopes.OwnerRule(),
			scopes.ChannelVisibilityQueryRule(scopes.ScopeReadChannels),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWriteChannels),
			scopes.ChannelManageOwnMutationRule(scopes.ScopeManageOwnModels),
		},
	}
}
