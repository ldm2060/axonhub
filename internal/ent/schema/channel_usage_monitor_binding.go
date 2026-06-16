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
)

type ChannelUsageMonitorBinding struct {
	ent.Schema
}

func (ChannelUsageMonitorBinding) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (ChannelUsageMonitorBinding) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("channel_id", "usage_monitor_channel_id", "deleted_at").
			StorageKey("channel_usage_monitor_bindings_unique_active").
			Unique(),
		index.Fields("channel_id", "deleted_at").
			StorageKey("channel_usage_monitor_bindings_by_channel"),
		index.Fields("usage_monitor_channel_id", "deleted_at").
			StorageKey("channel_usage_monitor_bindings_by_monitor"),
	}
}

func (ChannelUsageMonitorBinding) Fields() []ent.Field {
	return []ent.Field{
		field.Int("channel_id").Immutable(),
		field.Int("usage_monitor_channel_id").Immutable(),
		field.Bool("enabled").Default(true),
		field.Strings("trigger_statuses").
			Default([]string{}).
			Comment("Which quota statuses trigger this binding (e.g. exhausted, warning)"),
		field.JSON("conditions", []objects.QuotaMonitorBindingCondition{}).
			Default([]objects.QuotaMonitorBindingCondition{}).
			Comment("Structured conditions for triggering; evaluated against monitor poll data"),
		field.Time("last_triggered_at").
			Optional().
			Nillable(),
		field.String("last_trigger_reason").
			Optional().
			Nillable(),
	}
}

func (ChannelUsageMonitorBinding) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("channel", Channel.Type).
			Ref("quota_monitor_bindings").
			Field("channel_id").
			Unique().
			Required().
			Immutable(),
		edge.From("usage_monitor_channel", UsageMonitorChannel.Type).
			Ref("channel_bindings").
			Field("usage_monitor_channel_id").
			Unique().
			Required().
			Immutable(),
	}
}

func (ChannelUsageMonitorBinding) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}
