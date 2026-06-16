package schema

import (
	"entgo.io/contrib/entgql"
	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"

	"github.com/ldm2060/axonhub/internal/ent/schema/schematype"
)

type UsageMonitorChannel struct {
	ent.Schema
}

func (UsageMonitorChannel) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (UsageMonitorChannel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("channel_id"),
		index.Fields("status"),
		index.Fields("quota_status"),
	}
}

func (UsageMonitorChannel) Fields() []ent.Field {
	return []ent.Field{
		field.String("name").NotEmpty().
			Comment("Display name for this monitoring channel"),
		field.Enum("source").
			Values("builtin", "custom", "template").
			Comment("builtin: linked to existing Channel; custom: fully manual; template: from provider template"),
		field.Enum("provider_type").
			Values("claudecode", "codex", "github_copilot", "nanogpt", "wafer", "synthetic", "neuralwatt", "zhipu", "antigravity", "apertis").
			Optional().
			Comment("Provider type for quota template (required when source=template)"),
		field.Int("channel_id").Optional().Nillable().
			Comment("FK to Channel table (required when source=builtin)"),
		field.String("api_url").NotEmpty().
			Comment("API endpoint for quota query"),
		field.Enum("api_method").
			Values("GET", "POST").
			Default("GET").
			Comment("HTTP method for the quota API"),
		field.JSON("api_headers", map[string]any{}).
			Comment("HTTP request headers as JSON"),
		field.Text("api_body").Optional().
			Comment("Request body template for POST"),
		field.String("api_key").Optional().Sensitive().
			Comment("API key for authenticating with the provider (sensitive, hidden from GraphQL)"),
		field.Int("poll_interval").Default(300).
			Comment("Polling interval in seconds"),
		field.JSON("fields", []map[string]any{}).
			Comment("Array of field configurations").
			Annotations(entgql.Type("Map")),
		field.JSON("variables", []map[string]any{}).
			Annotations(entgql.Type("Map"), entgql.Skip(entgql.SkipAll)).
			Optional().
			Default([]map[string]any{}).
			Comment("Variable extraction rules"),
		field.JSON("display_fields", []map[string]any{}).
			Annotations(entgql.Type("Map"), entgql.Skip(entgql.SkipAll)).
			Optional().
			Default([]map[string]any{}).
			Comment("Display field definitions with optional badge styling"),
		field.Time("last_poll_at").Optional().Nillable().
			Comment("Last successful poll timestamp"),
		field.JSON("last_poll_data", map[string]any{}).Optional().
			Comment("Last poll parsed data for degraded display"),
		field.String("last_poll_error").Optional().Nillable().
			Comment("Last poll error message"),
		field.Enum("status").
			Values("active", "paused", "error").
			Default("active").
			Comment("Current status of this monitor channel"),
		field.Enum("quota_status").
			Values("available", "warning", "exhausted", "unknown").
			Optional().
			Comment("Derived quota status for orchestrator routing"),
		field.Bool("quota_ready").Optional().Nillable().Default(true).
			Comment("Whether channel is ready for routing based on quota status"),
		field.JSON("quota_limits", []map[string]any{}).Optional().
			Comment("Per-limit-type quota status for orchestrator routing (token/time/image)").
			Annotations(entgql.Type("Map")),
		field.Time("next_reset_at").Optional().Nillable().
			Comment("Earliest quota reset time across all limits"),
		field.Bool("auto_disable_enabled").
			Default(false).
			Comment("Enable automatic channel disabling based on quota status"),
		field.Float("auto_disable_threshold").
			Default(1.0).
			Optional().
			Comment("Disable channel when max usage ratio >= this threshold (0.0-1.0). Only used when auto_disable_enabled=true"),
		field.Float("auto_enable_threshold").
			Default(0.95).
			Optional().
			Comment("Re-enable channel when max usage ratio < this threshold (0.0-1.0). Only used when auto_disable_enabled=true"),
	}
}

func (UsageMonitorChannel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("channel", Channel.Type).
			Ref("usage_monitor_channels").
			Field("channel_id").
			Unique(),
		edge.From("owner", User.Type).
			Ref("usage_monitor_channels").
			Unique().
			Required(),
		edge.To("channel_bindings", ChannelUsageMonitorBinding.Type),
	}
}
