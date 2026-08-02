package schema

import (
	"entgo.io/ent"
	"entgo.io/ent/dialect/entsql"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/ldm2060/axonhub/internal/ent/privacy"
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
		edge.To("user", User.Type).
			Field("user_id").
			Unique().
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
		index.Fields("type", "email").Unique(),
		index.Fields("type", "email", "expires_at"),
	}
}

// Policy 定义 EmailToken 的权限策略.
// 邮件令牌是密码重置/邮箱验证的凭证，任何主体都不应通过 API 读取或写入，
// 只允许 EmailTokenService 通过 authz 系统旁路访问，因此这里一律拒绝。
func (EmailToken) Policy() ent.Policy {
	return privacy.Policy{
		Query: privacy.QueryPolicy{
			privacy.AlwaysDenyRule(),
		},
		Mutation: privacy.MutationPolicy{
			privacy.AlwaysDenyRule(),
		},
	}
}
