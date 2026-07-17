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
      type: 'openai';
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
