package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

type UserUsageStats struct {
	ent.Schema
}

func (UserUsageStats) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entsql.Annotation{Table: "user_usage_stats"},
	}
}

func (UserUsageStats) Fields() []ent.Field {
	return []ent.Field{
		field.Int("user_id").Immutable(),
		field.Int("request_count").Default(0),
		field.Int("success_count").Default(0),
		field.Int64("prompt_tokens").Default(0),
		field.Int64("completion_tokens").Default(0),
		field.Int64("total_tokens").Default(0),
		field.Float("total_cost").Default(0),
		field.Time("last_active_at").Optional().Nillable(),
	}
}

func (UserUsageStats) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("user_usage_stats").
			Field("user_id").
			Required().
				Unique().
				Immutable().
			Annotations(entgql.Directives(forceResolver())),
	}
}

func (UserUsageStats) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("user_id").Unique(),
	}
}

func (UserUsageStats) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}
