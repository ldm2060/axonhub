import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';

// Channel pooling (拼车) data layer: the viewer-facing pool quota list that
// feeds the battery popover, and the owner-facing per-channel pool status for
// the settings panel. Status values mirror the backend pooling constants.
export const POOL_STATUS = {
  available: 'available',
  warning: 'warning',
  paused: 'paused',
  unavailable: 'unavailable',
} as const;

export type PoolStatus = (typeof POOL_STATUS)[keyof typeof POOL_STATUS];

export interface MyPoolQuota {
  channelId: number;
  channelName: string;
  window: string;
  sharePct: number;
  memberQuota: number;
  used: number;
  totalQuota: number | null;
  periodStart: string | null;
  nextResetAt: string | null;
  status: string;
}

export interface PoolMemberStatus {
  userId: number;
  user: { id: string; email: string; firstName: string | null; lastName: string | null } | null;
  sharePct: number;
  quota: number | null;
  used: number;
  status: string;
}

export interface ChannelPoolStatus {
  channelId: number;
  enabled: boolean;
  window: string | null;
  totalQuota: number | null;
  periodStart: string | null;
  nextResetAt: string | null;
  estimateAvailable: boolean;
  ownerReservedPct: number;
  members: PoolMemberStatus[];
}

const MY_POOL_QUOTAS_QUERY = `
  query MyPoolQuotas {
    myPoolQuotas {
      channelId
      channelName
      window
      sharePct
      memberQuota
      used
      totalQuota
      periodStart
      nextResetAt
      status
    }
  }
`;

const CHANNEL_POOL_STATUS_QUERY = `
  query ChannelPoolStatus($channelId: ID!) {
    channelPoolStatus(channelID: $channelId) {
      channelId
      enabled
      window
      totalQuota
      periodStart
      nextResetAt
      estimateAvailable
      ownerReservedPct
      members {
        userId
        user {
          id
          email
          firstName
          lastName
        }
        sharePct
        quota
        used
        status
      }
    }
  }
`;

// The popover recomputes countdowns from its own polls; a minute keeps pool
// numbers as fresh as the monitor estimates they are derived from without
// hammering the usage aggregation.
export const POOL_QUOTAS_REFETCH_INTERVAL_MS = 60_000;

export function useMyPoolQuotas(options?: { refetchInterval?: number }) {
  return useQuery({
    queryKey: ['my-pool-quotas'],
    queryFn: async () => {
      const data = await graphqlRequest<{ myPoolQuotas: MyPoolQuota[] }>(MY_POOL_QUOTAS_QUERY);
      return data.myPoolQuotas;
    },
    refetchInterval: options?.refetchInterval ?? POOL_QUOTAS_REFETCH_INTERVAL_MS,
  });
}

export function useChannelPoolStatus(channelId?: string | null, enabled = true) {
  return useQuery({
    queryKey: ['channel-pool-status', channelId],
    queryFn: async () => {
      const data = await graphqlRequest<{ channelPoolStatus: ChannelPoolStatus }>(CHANNEL_POOL_STATUS_QUERY, {
        channelId,
      });
      return data.channelPoolStatus;
    },
    enabled: enabled && Boolean(channelId),
  });
}
