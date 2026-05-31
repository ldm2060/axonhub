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
			Values("claudecode", "codex", "github_copilot", "nanogpt", "wafer", "synthetic", "neuralwatt", "zhipu").
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
	}
}
