import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';

const CHECK_PROVIDER_QUOTAS_QUERY = `
  mutation CheckProviderQuotas {
    checkProviderQuotas
  }
`;

export async function checkProviderQuotas() {
  return graphqlRequest(CHECK_PROVIDER_QUOTAS_QUERY);
}

/** A parsed field value returned from the usage monitor channel. */
export type ParsedField = {
  key: string;
  label: string;
  value: string | null;
  total: string | null | undefined;
  percent: number | undefined;
  unit: string | undefined;
  format: string;
  error: string | null | undefined;
};

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
        lastPollError: ch.lastPollError ?? null,
      }),
    );
}
