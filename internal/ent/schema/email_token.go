package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
)

// EmailToken holds the schema definition for the EmailToken entity.
type EmailToken struct {
	ent.Schema
}

func (EmailToken) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
	}
}

// Fields of the EmailToken.
func (EmailToken) Fields() []ent.Field {
	return []ent.Field{
		field.String("token").Unique().NotEmpty(),
		field.Enum("type").Values("verify_email", "reset_password"),
		field.String("email").Optional().Nillable(),
		field.Time("expires_at"),
		field.Time("consumed_at").Optional().Nillable(),
		field.Int("user_id").Optional().Nillable(),
	}
}

// Edges of the EmailToken.
func (EmailToken) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("user", User.Type).
			Ref("email_tokens").
			Unique().
			Field("user_id").
			Annotations(
				entsql.OnDelete(entsql.Cascade),
			),
	}
}

// Indexes of the EmailToken.
func (EmailToken) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("token"),
		index.Fields("type", "expires_at"),
		index.Fields("type", "email", "expires_at"),
	}
}
