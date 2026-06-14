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

type Channel struct {
	ent.Schema
}

func (Channel) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		schematype.SoftDeleteMixin{},
	}
}

func (Channel) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("name", "deleted_at").
			StorageKey("channels_by_name").
			Unique(),
		index.Fields("owner_id", "deleted_at").StorageKey("channels_by_owner_id"),
	}
}

func (Channel) Fields() []ent.Field {
	return []ent.Field{
		field.Enum("type").
			Values(
				"openai",
				"openai_responses",
				"atlascloud",
				"codex",
				"vercel",
				"anthropic",
				"anthropic_aws",
				"anthropic_gcp",
				"gemini_openai",
				"gemini",
				"gemini_vertex",
				"deepseek",
				"deepseek_anthropic",
				"deepinfra",
				"qiniu",
				"fireworks",
				"doubao",
				"doubao_anthropic",
				"moonshot",
				"moonshot_anthropic",
				"zhipu",
				"zai",
				"zhipu_anthropic",
				"zai_anthropic",
				"anthropic_fake",
				"openai_fake",
				"openrouter",
				"xiaomi",
				"xiaomi_anthropic",
				"xai",
				"ppio",
				"siliconflow",
				"volcengine",
				"volcengine_anthropic",
				"longcat",
				"longcat_anthropic",
				"minimax",
				"minimax_anthropic",
				"aihubmix",
				"aihubmix_anthropic",
				"burncloud",
				"modelscope",
				"bailian",
				"bailian_anthropic",
				"moonshot_coding",
				"jina",
				"github",
				"github_copilot",
				"claudecode",
				"cerebras",
				"antigravity",
				"nanogpt",
				"nanogpt_responses",
				"opencode_go",
				"opencode_go_anthropic",
				"ollama",
				"evolink",
				"evolink_anthropic",
			).
			Annotations(
				entgql.OrderField("TYPE"),
			),
		field.String("base_url").Optional(),
		field.String("name").
			Annotations(
				entgql.OrderField("NAME"),
			),
		field.Enum("status").Values("enabled", "disabled", "archived").Default("disabled").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput),
				entgql.OrderField("STATUS"),
			),
		field.JSON("credentials", objects.ChannelCredentials{}).Sensitive(),
		field.JSON("disabled_api_keys", []objects.DisabledAPIKey{}).
			Default([]objects.DisabledAPIKey{}).
			Optional().
			Sensitive().
			Comment("Disabled API keys with metadata (sensitive; requires channel write permission)").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		field.Strings("supported_models"),
		field.Strings("manual_models").Optional().Default([]string{}),
		field.Bool("auto_sync_supported_models").Default(false),
		field.String("auto_sync_model_pattern").Optional().Default("").
			Comment("Regex pattern to filter models during auto-sync. Empty string means no filtering."),
		field.Strings("tags").Optional().Default([]string{}),
		field.String("default_test_model"),
		field.JSON("policies", objects.ChannelPolicies{}).
			Default(objects.ChannelPolicies{
				Stream: objects.CapabilityPolicyUnlimited,
			}).
			Annotations(
				entgql.Directives(forceResolver()),
			).
			Optional(),
		field.JSON("settings", &objects.ChannelSettings{}).
			Default(&objects.ChannelSettings{
				ModelMappings: []objects.ModelMapping{},
			}).Optional().
			Annotations(
				entgql.Directives(forceResolver()),
			),
		field.Int("ordering_weight").Default(0).Comment("Ordering weight for display sorting").
			Annotations(
				entgql.OrderField("ORDERING_WEIGHT"),
			),
		field.String("error_message").
			Optional().Nillable().
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput),
			),
		field.String("remark").
			Optional().Nillable().
			Comment("User-defined remark or note for the channel"),
		field.JSON("endpoints", []objects.ChannelEndpoint{}).
			Default([]objects.ChannelEndpoint{}).
			Optional().
			Comment("Outbound API endpoints for this channel. Each endpoint specifies api_format and optional path. When empty, defaults are derived from channel type."),
		field.Enum("client_restriction").
			Values("off", "lenient", "strict").
			Optional().
			Nillable().
			Comment("Client access restriction level. nil = inherit global, non-nil = override global. Only effective for coding channels.").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput),
			),
		field.JSON("auto_disable_config", &objects.ChannelAutoDisableConfig{}).
			Optional().
			Comment("Channel-level auto-disable configuration. nil = inherit global settings.").
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput),
			),
		field.Int("owner_id").Optional().Immutable().Annotations(entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput)),
		field.Enum("visibility").Values("private", "shared", "published").Default("private").Annotations(entgql.OrderField("VISIBILITY")),
		field.JSON("shared_with", []int{}).Optional().Annotations(entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput)),
		field.Bool("quota_binding_ready").
			Default(true).
			Comment("Aggregated quota-ready status from all bound UsageMonitorChannels. When false, channel is excluded from routing."),
		field.Enum("quota_multi_monitor_strategy").
			Values("any", "all").
			Optional().
			Nillable().
			Comment("Strategy for aggregating quota status from multiple bound monitors: 'any'=disable if any exhausted, 'all'=disable if all exhausted. nil = use global default."),
	}
}

func (Channel) Edges() []ent.Edge {
	return []ent.Edge{
		edge.From("owner", User.Type).Ref("owned_channels").Unique().Immutable().Field("owner_id").Annotations(entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput)),
		edge.To("requests", Request.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("executions", RequestExecution.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("usage_logs", UsageLog.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
				entgql.RelayConnection(),
			),
		edge.To("channel_probes", ChannelProbe.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		edge.To("channel_model_prices", ChannelModelPrice.Type).
			Annotations(
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		edge.To("provider_quota_status", ProviderQuotaStatus.Type).
			Unique().
			Annotations(
				entgql.Directives(forceResolver()),
				entgql.Skip(entgql.SkipMutationCreateInput, entgql.SkipMutationUpdateInput),
			),
		edge.To("usage_monitor_channels", UsageMonitorChannel.Type),
	}
}

func (Channel) Annotations() []schema.Annotation {
	return []schema.Annotation{
		entgql.QueryField(),
		entgql.RelayConnection(),
		entgql.Mutations(entgql.MutationCreate(), entgql.MutationUpdate()),
	}
}

// Policy 定义 Channel 的权限策略.
func (Channel) Policy() ent.Policy {
	return scopes.Policy{
		Query: scopes.QueryPolicy{
			scopes.APIKeyScopeQueryRule(scopes.ScopeReadChannels),
			scopes.OwnerRule(),
			scopes.ChannelVisibilityQueryRule(scopes.ScopeReadChannels),
		},
		Mutation: scopes.MutationPolicy{
			scopes.OwnerRule(),
			scopes.UserWriteScopeRule(scopes.ScopeWriteChannels),
			scopes.ChannelManageOwnMutationRule(scopes.ScopeManageOwnChannels),
		},
	}
}
