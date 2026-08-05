import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import type { ParsedField, DisplayField } from '@/features/usage-monitor/data/schema';

const CHECK_PROVIDER_QUOTAS_QUERY = `
  mutation CheckProviderQuotas {
    checkProviderQuotas
  }
`;

export async function checkProviderQuotas() {
  return graphqlRequest(CHECK_PROVIDER_QUOTAS_QUERY);
}

/** Simplified quota channel shape for badge rendering. */
export type QuotaChannel = {
  id: string;
  name: string;
  providerType: string | null;
  channelName: string | null;
  channelType: string | null;
  quotaStatus: 'available' | 'warning' | 'exhausted' | 'unknown' | null;
  quotaReady: boolean | null;
  nextResetAt: string | null;
  parsedData: ParsedField[];
  displayFields: DisplayField[];
  lastPollError: string | null;
};

const QUOTA_USAGE_MONITOR_CHANNELS_QUERY = `
  query QuotaUsageMonitorChannels {
    usageMonitorChannelsList {
      id
      name
      source
      providerType
      channelID
      status
      lastPollError
      quotaStatus
      quotaReady
      nextResetAt
      channel {
        id
        name
        type
      }
      parsedData {
        key
        label
        value
        total
        percent
        unit
        format
        error
        group
        groupLabel
      }
      displayFields {
        key
        label
        valueRef
        format
        unit
        totalRef
        displayOrder
        badge
        badgePresets
        group
        groupLabelRef
      }
    }
  }
`;

export function useQuotaChannels(): QuotaChannel[] {
  const { data } = useQuery({
    queryKey: ['quota-usage-monitor-channels'],
    queryFn: async () => {
      const result = await graphqlRequest<{
        usageMonitorChannelsList: any[];
      }>(QUOTA_USAGE_MONITOR_CHANNELS_QUERY);
      return result.usageMonitorChannelsList ?? [];
    },
    refetchInterval: 60_000,
    refetchIntervalInBackground: true,
  });

  if (!data) return [];

  // Filter to only channels that have quota status set
  return data
    .filter((ch: any) => ch.quotaStatus != null)
    .map(
      (ch: any): QuotaChannel => ({
        id: ch.id,
        name: ch.name,
        providerType: ch.providerType ?? null,
        channelName: ch.channel?.name ?? null,
        channelType: ch.channel?.type ?? null,
        quotaStatus: ch.quotaStatus ?? null,
        quotaReady: ch.quotaReady ?? null,
        nextResetAt: ch.nextResetAt ?? null,
        parsedData: (ch.parsedData ?? []) as ParsedField[],
        displayFields: (ch.displayFields ?? []) as DisplayField[],
        lastPollError: ch.lastPollError ?? null,
      })
    );
}

export type ProviderQuotaDataCommon = {
  plan_type?: string;
  error?: string;
};

export type ProviderClaudeQuotaData = ProviderQuotaDataCommon & {
  windows?: {
    '5h'?: { utilization?: number; reset?: number; status?: string };
    '7d'?: { utilization?: number; reset?: number; status?: string };
    overage?: { utilization?: number; reset?: number; status?: string };
  };
  representative_claim?: string;
};

export type ProviderCodexQuotaData = ProviderQuotaDataCommon & {
  rate_limit?: {
    primary_window?: {
      used_percent?: number;
      reset_at?: number;
      reset_after_seconds?: number;
      limit_window_seconds?: number;
    };
    secondary_window?: {
      used_percent?: number;
      reset_at?: number;
      reset_after_seconds?: number;
      limit_window_seconds?: number;
    };
  };
};

export type CopilotQuotaSnapshot = {
  entitlement: number;
  has_quota: boolean;
  overage_count: number;
  overage_permitted: boolean;
  percent_remaining: number;
  quota_id: string;
  quota_remaining: number;
  quota_reset_at: number;
  remaining: number;
  timestamp_utc: string;
  unlimited: boolean;
};

export type ProviderGitHubCopilotQuotaData = ProviderQuotaDataCommon & {
  limited_user_quotas?: {
    chat?: number;
    completions?: number;
    [key: string]: number | undefined;
  };
  quota_snapshots?: {
    chat?: CopilotQuotaSnapshot;
    completions?: CopilotQuotaSnapshot;
    premium_interactions?: CopilotQuotaSnapshot;
    premium_models?: CopilotQuotaSnapshot;
    [key: string]: CopilotQuotaSnapshot | undefined;
  };
  total_quotas?: {
    chat?: number;
    completions?: number;
    [key: string]: number | undefined;
  };
};

export type NanoGPTQuotaWindow = {
  used?: number;
  remaining?: number;
  percentUsed?: number;
  resetAt?: number;
};

export type ProviderNanoGPTQuotaData = ProviderQuotaDataCommon & {
  state?: string;
  active?: boolean;
  allowOverage?: boolean;
  limits?: {
    weeklyInputTokens?: number;
    dailyImages?: number;
    dailyInputTokens?: number;
  };
  windows?: {
    weeklyInputTokens?: NanoGPTQuotaWindow | null;
    dailyImages?: NanoGPTQuotaWindow | null;
    dailyInputTokens?: NanoGPTQuotaWindow | null;
  };
  period?: { currentPeriodEnd?: string };
};

export type ProviderWaferQuotaData = ProviderQuotaDataCommon & {
  current_period_used_percent?: number | null;
  remaining_included_requests?: number | null;
  included_request_limit?: number | null;
  overage_request_count?: number | null;
  window_start?: string | null;
  window_end?: string | null;
  plan_tier?: string | null;
};

export type ProviderSyntheticQuotaData = ProviderQuotaDataCommon & {
  weeklyTokenLimit?: {
    percentRemaining?: number | null;
    remainingCredits?: string | null;
    maxCredits?: string | null;
    nextRegenAt?: string | null;
  } | null;
  rollingFiveHourLimit?: {
    limited?: boolean | null;
    remaining?: number | null;
    max?: number | null;
    nextTickAt?: string | null;
    tickPercent?: number | null;
  } | null;
};

export type ProviderNeuralWattQuotaData = ProviderQuotaDataCommon & {
  balance?: {
    credits_remaining_usd?: number | null;
    total_credits_usd?: number | null;
  } | null;
  subscription?: {
    kwh_included?: number | null;
    kwh_used?: number | null;
    kwh_remaining?: number | null;
    in_overage?: boolean | null;
    status?: string | null;
    plan?: string | null;
    kwh_reset_date?: string | null;
  } | null;
};

export type OpenCodeGoQuotaWindow = {
  usage_percent?: number;
  reset_in_seconds?: number;
  reset_time?: string;
  status?: string;
  percent_remaining?: number;
};

export type ProviderOpenCodeGoQuotaData = ProviderQuotaDataCommon & {
  windows?: {
    rolling?: OpenCodeGoQuotaWindow;
    weekly?: OpenCodeGoQuotaWindow;
    monthly?: OpenCodeGoQuotaWindow;
  };
};

export type KimiCodeUsageRow = {
  label: string;
  used: number;
  limit: number;
  resetAt?: string;
  resetAfterSeconds?: number;
};

export type ProviderKimiCodeQuotaData = ProviderQuotaDataCommon & {
  rows?: KimiCodeUsageRow[];
  boosterWallet?: {
    balanceCents: number;
    totalCents: number;
    monthlyChargeLimitEnabled: boolean;
    monthlyChargeLimitCents: number;
    monthlyUsedCents: number;
    currency: string;
  };
};

export type MinimaxModelRow = {
  modelName: string;
  intervalUsedPercent: number;
  intervalTotalPercent: number;
  intervalPercent: number;
  intervalStatus: string;
  intervalResetAt?: string;
  weeklyUsedPercent: number;
  weeklyTotalPercent: number;
  weeklyPercent: number;
  weeklyStatus: string;
  weeklyResetAt?: string;
  weeklyBoostPermille?: number;
};

export type ProviderMinimaxQuotaData = ProviderQuotaDataCommon & {
  rows?: MinimaxModelRow[];
};

export type ZhipuWindowRow = {
  window: string;
  usedPercent: number;
  status: string;
  resetAt?: string;
};

export type ProviderZhipuQuotaData = ProviderQuotaDataCommon & {
  rows?: ZhipuWindowRow[];
  level?: string;
};

export type ClineQuotaWindow = {
  window_state?: 'active' | 'inactive' | 'unavailable' | 'invalid';
  active_window?: boolean;
  window_start_at?: string;
  cost_start_at?: string;
  items_count?: number;
  used_cost_units?: number;
  limit_cost_units: number;
  remaining_cost_units?: number;
  credits_used?: number;
  usage_ratio?: number;
  usage_percent?: number;
  cost_usage_ratio?: number;
  cost_usage_percent?: number;
  usage_source?: string;
  reset_source?: string;
  cost_source?: string;
  next_reset_at?: string | null;
};

type ClineBalance = {
  raw_balance?: number | null;
  unit_note?: string;
};

type ClineUsageFetch = {
  pages: number;
  items_seen: number;
  cline_pass_items_seen?: number;
  direct_items_seen?: number;
  unclassified_items_seen?: number;
  invalid_timestamp_items?: number;
  truncated: boolean;
};

type ProviderClinePassQuotaData = ProviderQuotaDataCommon & {
  model_scope: 'cline_pass_only' | 'mixed' | 'unknown';
  status_basis: string;
  pool: 'cline_pass';
  pool_note?: string;
  cost_scale: number;
  balance: ClineBalance;
  windows: {
    last5h: ClineQuotaWindow;
    last7d: ClineQuotaWindow;
    last30d: ClineQuotaWindow;
  };
  usage_fetch: ClineUsageFetch;
};

type ProviderClineUnavailablePassQuotaData = ProviderQuotaDataCommon & {
  model_scope: 'cline_pass_only' | 'mixed' | 'unknown';
  status_basis: 'cline_pass_unavailable' | 'cline_pass_unavailable_mixed_pool';
  pool: 'cline_pass';
  pool_note?: string;
  pass_state: 'unavailable';
  balance: ClineBalance;
  cost_scale?: never;
  windows?: never;
  usage_fetch?: never;
};

type ProviderClineDirectQuotaData = ProviderQuotaDataCommon & {
  model_scope: 'direct_only';
  status_basis: string;
  pool: 'direct_credit' | string;
  pool_note?: string;
  balance: ClineBalance;
  cost_scale?: never;
  windows?: never;
  usage_fetch?: never;
};

type ProviderClineErrorQuotaData = ProviderQuotaDataCommon & {
  model_scope?: undefined;
  status_basis?: string;
  pool?: string;
  balance?: ClineBalance;
  cost_scale?: never;
  windows?: never;
  usage_fetch?: never;
};

export type ProviderClineQuotaData =
  | ProviderClinePassQuotaData
  | ProviderClineUnavailablePassQuotaData
  | ProviderClineDirectQuotaData
  | ProviderClineErrorQuotaData;

export function isClineActivePassQuotaData(qd: ProviderClineQuotaData): qd is ProviderClinePassQuotaData {
  return qd.pool === 'cline_pass' && qd.windows != null;
}

export function isClineUnavailablePassQuotaData(qd: ProviderClineQuotaData): qd is ProviderClineUnavailablePassQuotaData {
  return 'pass_state' in qd && qd.pass_state === 'unavailable';
}

export type ProviderQuotaChannel = {
  id: string;
  name: string;
  quotaStatus?: {
    status: 'available' | 'warning' | 'exhausted' | 'unknown';
    nextResetAt: string | null;
    ready: boolean;
  };
} & (
  | {
      type: 'claudecode';
      quotaStatus?: {
        quotaData: ProviderClaudeQuotaData;
      };
    }
  | {
      type: 'codex';
      quotaStatus?: {
        quotaData: ProviderCodexQuotaData;
      };
    }
  | {
      type: 'github_copilot';
      quotaStatus?: {
        quotaData: ProviderGitHubCopilotQuotaData;
      };
    }
  | {
      type: 'nanogpt';
      quotaStatus?: {
        quotaData: ProviderNanoGPTQuotaData;
      };
    }
  | {
      type: 'nanogpt_responses';
      quotaStatus?: {
        quotaData: ProviderNanoGPTQuotaData;
      };
    }
  | {
      type: 'opencode_go' | 'opencode_go_anthropic';
      workspaceId?: string | null;
      quotaStatus?: {
        quotaData: ProviderOpenCodeGoQuotaData;
      };
    }
  | {
      type: 'moonshot_coding';
      quotaStatus?: {
        quotaData: ProviderKimiCodeQuotaData;
      };
    }
  | {
      type: 'minimax' | 'minimax_anthropic';
      quotaStatus?: {
        quotaData: ProviderMinimaxQuotaData;
      };
    }
  | {
      type: 'zhipu' | 'zhipu_anthropic';
      quotaStatus?: {
        quotaData: ProviderZhipuQuotaData;
      };
    }
  | {
      type: 'openai' | 'openai_responses';
      providerType: 'wafer';
      quotaStatus?: {
        quotaData: ProviderWaferQuotaData;
      };
    }
  | {
      type: 'openai';
      providerType: 'synthetic';
      quotaStatus?: {
        quotaData: ProviderSyntheticQuotaData;
      };
    }
  | {
      type: 'openai';
      providerType: 'neuralwatt';
      quotaStatus?: {
        quotaData: ProviderNeuralWattQuotaData;
      };
    }
  | {
      type: 'openai';
      providerType?: undefined;
      quotaStatus?: {
        quotaData: ProviderQuotaDataCommon;
      };
    }
);

export function useProviderQuotaStatuses(): QuotaChannel[] {
  return useQuotaChannels();
}
