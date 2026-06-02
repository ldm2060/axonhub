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
      }),
    );
}
