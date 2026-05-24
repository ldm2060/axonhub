import { z } from 'zod';
import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';

// Schemas
export const userUsageStatSchema = z.object({
  userID: z.number(),
  userName: z.string(),
  userEmail: z.string(),
  requestCount: z.number(),
  successCount: z.number(),
  successRate: z.number(),
  promptTokens: z.number(),
  completionTokens: z.number(),
  totalTokens: z.number(),
  totalCost: z.number(),
  lastActiveAt: z.string().nullable(),
});

export const userUsageStatsPayloadSchema = z.object({
  stats: z.array(userUsageStatSchema),
  totalUsers: z.number(),
  activeUsers7d: z.number(),
  activeUsers30d: z.number(),
});

// Types
export type UserUsageStat = z.infer<typeof userUsageStatSchema>;
export type UserUsageStatsPayload = z.infer<typeof userUsageStatsPayloadSchema>;

export type UserStatsSortField = 'REQUEST_COUNT' | 'TOTAL_COST' | 'TOTAL_TOKENS' | 'LAST_ACTIVE_AT';
export type TimeRange = 'LAST_7D' | 'LAST_30D' | 'ALL';

// GraphQL query
const USER_USAGE_STATS_QUERY = `
  query UserUsageStats(
    $timeRange: TimeRange!
    $search: String
    $sortBy: UserStatsSortField!
    $sortOrder: OrderDirection!
    $page: Int!
    $pageSize: Int!
  ) {
    userUsageStats(
      timeRange: $timeRange
      search: $search
      sortBy: $sortBy
      sortOrder: $sortOrder
      page: $page
      pageSize: $pageSize
    ) {
      stats {
        userID
        userName
        userEmail
        requestCount
        successCount
        successRate
        promptTokens
        completionTokens
        totalTokens
        totalCost
        lastActiveAt
      }
      totalUsers
      activeUsers7d
      activeUsers30d
    }
  }
`;

// Hook
export function useUserUsageStats(
  timeRange: TimeRange = 'ALL',
  search: string = '',
  sortBy: UserStatsSortField = 'REQUEST_COUNT',
  sortOrder: 'ASC' | 'DESC' = 'DESC',
  page: number = 1,
  pageSize: number = 20,
) {
  return useQuery({
    queryKey: ['userUsageStats', timeRange, search, sortBy, sortOrder, page, pageSize],
    queryFn: async () => {
      const data = await graphqlRequest<{
        userUsageStats: unknown
      }>(USER_USAGE_STATS_QUERY, {
        timeRange,
        search: search || null,
        sortBy,
        sortOrder,
        page,
        pageSize,
      });
      return userUsageStatsPayloadSchema.parse(data.userUsageStats);
    },
    refetchInterval: 300000,
    placeholderData: (previousData) => previousData,
  });
}
