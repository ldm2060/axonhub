import { z } from 'zod';
import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';

// Schema definitions
export type DashboardMode = 'project' | 'personal';

export const requestStatsSchema = z.object({
  requestsToday: z.number(),
  requestsThisWeek: z.number(),
  requestsLastWeek: z.number(),
  requestsThisMonth: z.number(),
});

export const dashboardStatsSchema = z.object({
  totalRequests: z.number(),
  requestStats: requestStatsSchema,
  failedRequests: z.number(),
  averageResponseTime: z.number().nullable(),
});

export const requestsByChannelSchema = z.object({
  channelName: z.string(),
  count: z.number(),
});

export const requestsByModelSchema = z.object({
  modelId: z.string(),
  count: z.number(),
});

export const requestsByAPIKeySchema = z.object({
  apiKeyId: z.string(),
  apiKeyName: z.string(),
  count: z.number(),
});

export const tokensByAPIKeySchema = z.object({
  apiKeyId: z.string(),
  apiKeyName: z.string(),
  inputTokens: z.number(),
  outputTokens: z.number(),
  cachedTokens: z.number(),
  reasoningTokens: z.number(),
  totalTokens: z.number(),
});

export const tokensByChannelSchema = z.object({
  channelId: z.string(),
  channelName: z.string(),
  inputTokens: z.number(),
  outputTokens: z.number(),
  cachedTokens: z.number(),
  reasoningTokens: z.number(),
  totalTokens: z.number(),
});

export const tokensByModelSchema = z.object({
  modelId: z.string(),
  inputTokens: z.number(),
  outputTokens: z.number(),
  cachedTokens: z.number(),
  reasoningTokens: z.number(),
  totalTokens: z.number(),
});

export const costByChannelSchema = z.object({
  channelName: z.string(),
  cost: z.number(),
});

export const costByModelSchema = z.object({
  modelId: z.string(),
  cost: z.number(),
});

export const costByAPIKeySchema = z.object({
  apiKeyId: z.string(),
  apiKeyName: z.string(),
  cost: z.number(),
});

export const dailyRequestStatsSchema = z.object({
  date: z.string(),
  count: z.number(),
  tokens: z.number(),
  cost: z.number(),
});

export const channelSuccessRateSchema = z.object({
  channelId: z.string(),
  channelName: z.string(),
  channelType: z.string(),
  channelDisabled: z.boolean(),
  successCount: z.number(),
  failedCount: z.number(),
  totalCount: z.number(),
  successRate: z.number(),
});

export const modelPerformanceStatSchema = z.object({
  date: z.string(),
  modelId: z.string(),
  throughput: z.number().nullable(),
  ttftMs: z.number().nullable(),
  requestCount: z.number(),
});

export const channelPerformanceStatSchema = z.object({
  date: z.string(),
  channelId: z.string(),
  channelName: z.string(),
  throughput: z.number().nullable(),
  ttftMs: z.number().nullable(),
  requestCount: z.number(),
});

export type RequestStats = z.infer<typeof requestStatsSchema>;
export type DashboardStats = z.infer<typeof dashboardStatsSchema>;
export type RequestsByChannel = z.infer<typeof requestsByChannelSchema>;
export type RequestsByModel = z.infer<typeof requestsByModelSchema>;
export type RequestsByAPIKey = z.infer<typeof requestsByAPIKeySchema>;
export type TokensByAPIKey = z.infer<typeof tokensByAPIKeySchema>;
export type TokensByChannel = z.infer<typeof tokensByChannelSchema>;
export type TokensByModel = z.infer<typeof tokensByModelSchema>;
export type CostByChannel = z.infer<typeof costByChannelSchema>;
export type CostByModel = z.infer<typeof costByModelSchema>;
export type CostByAPIKey = z.infer<typeof costByAPIKeySchema>;
export type DailyRequestStats = z.infer<typeof dailyRequestStatsSchema>;
export type ChannelSuccessRate = z.infer<typeof channelSuccessRateSchema>;
export type ModelPerformanceStat = z.infer<typeof modelPerformanceStatSchema>;
export type ChannelPerformanceStat = z.infer<typeof channelPerformanceStatSchema>;

export const tokenStatsSchema = z.object({
  totalInputTokensToday: z.number(),
  totalOutputTokensToday: z.number(),
  totalCachedTokensToday: z.number(),
  totalInputTokensThisWeek: z.number(),
  totalOutputTokensThisWeek: z.number(),
  totalCachedTokensThisWeek: z.number(),
  totalInputTokensThisMonth: z.number(),
  totalOutputTokensThisMonth: z.number(),
  totalCachedTokensThisMonth: z.number(),
  totalInputTokensAllTime: z.number(),
  totalOutputTokensAllTime: z.number(),
  totalCachedTokensAllTime: z.number(),
  lastUpdated: z.string().nullable(),
});

export type TokenStats = z.infer<typeof tokenStatsSchema>;

// GraphQL queries - Admin
const DASHBOARD_STATS_QUERY = `
  query GetDashboardStats {
    dashboardOverview {
      totalRequests
      requestStats {
        requestsToday
        requestsThisWeek
        requestsLastWeek
        requestsThisMonth
      }
      failedRequests
      averageResponseTime
    }
  }
`;

const REQUESTS_BY_CHANNEL_QUERY = `
  query GetRequestsByChannel($timeWindow: String) {
    requestStatsByChannel(timeWindow: $timeWindow) {
      channelName
      count
    }
  }
`;

const REQUESTS_BY_MODEL_QUERY = `
  query GetRequestsByModel($timeWindow: String) {
    requestStatsByModel(timeWindow: $timeWindow) {
      modelId
      count
    }
  }
`;

const REQUESTS_BY_API_KEY_QUERY = `
  query GetRequestsByAPIKey($timeWindow: String) {
    requestStatsByAPIKey(timeWindow: $timeWindow) {
      apiKeyId
      apiKeyName
      count
    }
  }
`;

const TOKENS_BY_API_KEY_QUERY = `
  query GetTokensByAPIKey($timeWindow: String) {
    tokenStatsByAPIKey(timeWindow: $timeWindow) {
      apiKeyId
      apiKeyName
      inputTokens
      outputTokens
      cachedTokens
      reasoningTokens
      totalTokens
    }
  }
`;

const TOKENS_BY_CHANNEL_QUERY = `
  query GetTokensByChannel($timeWindow: String) {
    tokenStatsByChannel(timeWindow: $timeWindow) {
      channelId
      channelName
      inputTokens
      outputTokens
      cachedTokens
      reasoningTokens
      totalTokens
    }
  }
`;

const TOKENS_BY_MODEL_QUERY = `
  query GetTokensByModel($timeWindow: String) {
    tokenStatsByModel(timeWindow: $timeWindow) {
      modelId
      inputTokens
      outputTokens
      cachedTokens
      reasoningTokens
      totalTokens
    }
  }
`;

const COST_BY_CHANNEL_QUERY = `
  query GetCostByChannel($timeWindow: String) {
    costStatsByChannel(timeWindow: $timeWindow) {
      channelName
      cost
    }
  }
`;

const COST_BY_MODEL_QUERY = `
  query GetCostByModel($timeWindow: String) {
    costStatsByModel(timeWindow: $timeWindow) {
      modelId
      cost
    }
  }
`;

const COST_BY_API_KEY_QUERY = `
  query GetCostByAPIKey($timeWindow: String) {
    costStatsByAPIKey(timeWindow: $timeWindow) {
      apiKeyId
      apiKeyName
      cost
    }
  }
`;

const DAILY_REQUEST_STATS_QUERY = `
  query GetDailyRequestStats {
    dailyRequestStats {
      date
      count
      tokens
      cost
    }
  }
`;

const CHANNEL_SUCCESS_RATES_QUERY = `
  query GetChannelSuccessRates($timeWindow: String, $limit: Int) {
    channelSuccessRates(timeWindow: $timeWindow, limit: $limit) {
      channelId
      channelName
      channelType
      channelDisabled
      successCount
      failedCount
      totalCount
      successRate
    }
  }
`;

const MODEL_PERFORMANCE_STATS_QUERY = `
  query ModelPerformanceStats {
    modelPerformanceStats {
      date
      modelId
      throughput
      ttftMs
      requestCount
    }
  }
`;

const CHANNEL_PERFORMANCE_STATS_QUERY = `
  query ChannelPerformanceStats {
    channelPerformanceStats {
      date
      channelId
      channelName
      throughput
      ttftMs
      requestCount
    }
  }
`;

const TOKEN_STATS_AGGR_QUERY = `
  query GetTokenStats {
    tokenStats {
      totalInputTokensToday
      totalOutputTokensToday
      totalCachedTokensToday
      totalInputTokensThisWeek
      totalOutputTokensThisWeek
      totalCachedTokensThisWeek
      totalInputTokensThisMonth
      totalOutputTokensThisMonth
      totalCachedTokensThisMonth
      totalInputTokensAllTime
      totalOutputTokensAllTime
      totalCachedTokensAllTime
      lastUpdated
    }
  }
`;

// GraphQL queries - Personal (myXxx)
const MY_DASHBOARD_STATS_QUERY = `
  query GetMyDashboardStats {
    myDashboard {
      totalRequests
      requestStats {
        requestsToday
        requestsThisWeek
        requestsLastWeek
        requestsThisMonth
      }
      failedRequests
      averageResponseTime
    }
  }
`;

const MY_REQUESTS_BY_CHANNEL_QUERY = `
  query GetMyRequestsByChannel($timeWindow: String) {
    myRequestStatsByChannel(timeWindow: $timeWindow) {
      channelName
      count
    }
  }
`;

const MY_REQUESTS_BY_MODEL_QUERY = `
  query GetMyRequestsByModel($timeWindow: String) {
    myRequestStatsByModel(timeWindow: $timeWindow) {
      modelId
      count
    }
  }
`;

const MY_REQUESTS_BY_API_KEY_QUERY = `
  query GetMyRequestsByAPIKey($timeWindow: String) {
    myRequestStatsByAPIKey(timeWindow: $timeWindow) {
      apiKeyId
      apiKeyName
      count
    }
  }
`;

const MY_TOKENS_BY_API_KEY_QUERY = `
  query GetMyTokensByAPIKey($timeWindow: String) {
    myTokenStatsByAPIKey(timeWindow: $timeWindow) {
      apiKeyId
      apiKeyName
      inputTokens
      outputTokens
      cachedTokens
      reasoningTokens
      totalTokens
    }
  }
`;

const MY_TOKENS_BY_CHANNEL_QUERY = `
  query GetMyTokensByChannel($timeWindow: String) {
    myTokenStatsByChannel(timeWindow: $timeWindow) {
      channelId
      channelName
      inputTokens
      outputTokens
      cachedTokens
      reasoningTokens
      totalTokens
    }
  }
`;

const MY_TOKENS_BY_MODEL_QUERY = `
  query GetMyTokensByModel($timeWindow: String) {
    myTokenStatsByModel(timeWindow: $timeWindow) {
      modelId
      inputTokens
      outputTokens
      cachedTokens
      reasoningTokens
      totalTokens
    }
  }
`;

const MY_COST_BY_CHANNEL_QUERY = `
  query GetMyCostByChannel($timeWindow: String) {
    myCostStatsByChannel(timeWindow: $timeWindow) {
      channelName
      cost
    }
  }
`;

const MY_COST_BY_MODEL_QUERY = `
  query GetMyCostByModel($timeWindow: String) {
    myCostStatsByModel(timeWindow: $timeWindow) {
      modelId
      cost
    }
  }
`;

const MY_COST_BY_API_KEY_QUERY = `
  query GetMyCostByAPIKey($timeWindow: String) {
    myCostStatsByAPIKey(timeWindow: $timeWindow) {
      apiKeyId
      apiKeyName
      cost
    }
  }
`;

const MY_DAILY_REQUEST_STATS_QUERY = `
  query GetMyDailyRequestStats {
    myDailyRequestStats {
      date
      count
      tokens
      cost
    }
  }
`;

const MY_CHANNEL_SUCCESS_RATES_QUERY = `
  query GetMyChannelSuccessRates($timeWindow: String, $limit: Int) {
    myChannelSuccessRates(timeWindow: $timeWindow, limit: $limit) {
      channelId
      channelName
      channelType
      channelDisabled
      successCount
      failedCount
      totalCount
      successRate
    }
  }
`;

const MY_MODEL_PERFORMANCE_STATS_QUERY = `
  query MyModelPerformanceStats {
    myModelPerformanceStats {
      date
      modelId
      throughput
      ttftMs
      requestCount
    }
  }
`;

const MY_CHANNEL_PERFORMANCE_STATS_QUERY = `
  query MyChannelPerformanceStats {
    myChannelPerformanceStats {
      date
      channelId
      channelName
      throughput
      ttftMs
      requestCount
    }
  }
`;

const MY_TOKEN_STATS_AGGR_QUERY = `
  query GetMyTokenStats {
    myTokenStats {
      totalInputTokensToday
      totalOutputTokensToday
      totalCachedTokensToday
      totalInputTokensThisWeek
      totalOutputTokensThisWeek
      totalCachedTokensThisWeek
      totalInputTokensThisMonth
      totalOutputTokensThisMonth
      totalCachedTokensThisMonth
      totalInputTokensAllTime
      totalOutputTokensAllTime
      totalCachedTokensAllTime
      lastUpdated
    }
  }
`;

// Query hooks
export function useDashboardStats(mode: DashboardMode = 'project') {
  const isPersonal = mode === 'personal';
  return useQuery({
    queryKey: ['dashboardStats', isPersonal],
    queryFn: async () => {
      const query = isPersonal ? MY_DASHBOARD_STATS_QUERY : DASHBOARD_STATS_QUERY;
      const fieldName = isPersonal ? 'myDashboard' : 'dashboardOverview';
      const data = await graphqlRequest<{ [key: string]: DashboardStats }>(query);
      return dashboardStatsSchema.parse(data[fieldName]);
    },
    refetchInterval: 30000,
  });
}

export function useRequestsByChannel(timeWindow?: string, mode: DashboardMode = 'project') {
  const isPersonal = mode === 'personal';
  return useQuery({
    queryKey: ['requestStatsByChannel', timeWindow, isPersonal],
    queryFn: async () => {
      const query = isPersonal ? MY_REQUESTS_BY_CHANNEL_QUERY : REQUESTS_BY_CHANNEL_QUERY;
      const fieldName = isPersonal ? 'myRequestStatsByChannel' : 'requestStatsByChannel';
      const data = await graphqlRequest<{ [key: string]: RequestsByChannel[] }>(query, { timeWindow });
      return data[fieldName].map((item) => requestsByChannelSchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

export function useRequestsByModel(timeWindow?: string, mode: DashboardMode = 'project') {
  const isPersonal = mode === 'personal';
  return useQuery({
    queryKey: ['requestStatsByModel', timeWindow, isPersonal],
    queryFn: async () => {
      const query = isPersonal ? MY_REQUESTS_BY_MODEL_QUERY : REQUESTS_BY_MODEL_QUERY;
      const fieldName = isPersonal ? 'myRequestStatsByModel' : 'requestStatsByModel';
      const data = await graphqlRequest<{ [key: string]: RequestsByModel[] }>(query, { timeWindow });
      return data[fieldName].map((item) => requestsByModelSchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

export function useRequestsByAPIKey(timeWindow?: string, mode: DashboardMode = 'project') {
  const isPersonal = mode === 'personal';
  return useQuery({
    queryKey: ['requestStatsByAPIKey', timeWindow, isPersonal],
    queryFn: async () => {
      const query = isPersonal ? MY_REQUESTS_BY_API_KEY_QUERY : REQUESTS_BY_API_KEY_QUERY;
      const fieldName = isPersonal ? 'myRequestStatsByAPIKey' : 'requestStatsByAPIKey';
      const data = await graphqlRequest<{ [key: string]: RequestsByAPIKey[] }>(query, { timeWindow });
      return data[fieldName].map((item) => requestsByAPIKeySchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

export function useTokensByAPIKey(timeWindow?: string, mode: DashboardMode = 'project') {
  const isPersonal = mode === 'personal';
  return useQuery({
    queryKey: ['tokenStatsByAPIKey', timeWindow, isPersonal],
    queryFn: async () => {
      const query = isPersonal ? MY_TOKENS_BY_API_KEY_QUERY : TOKENS_BY_API_KEY_QUERY;
      const fieldName = isPersonal ? 'myTokenStatsByAPIKey' : 'tokenStatsByAPIKey';
      const data = await graphqlRequest<{ [key: string]: TokensByAPIKey[] }>(query, { timeWindow });
      return data[fieldName].map((item) => tokensByAPIKeySchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

export function useTokensByChannel(timeWindow?: string, mode: DashboardMode = 'project') {
  const isPersonal = mode === 'personal';
  return useQuery({
    queryKey: ['tokenStatsByChannel', timeWindow, isPersonal],
    queryFn: async () => {
      const query = isPersonal ? MY_TOKENS_BY_CHANNEL_QUERY : TOKENS_BY_CHANNEL_QUERY;
      const fieldName = isPersonal ? 'myTokenStatsByChannel' : 'tokenStatsByChannel';
      const data = await graphqlRequest<{ [key: string]: TokensByChannel[] }>(query, { timeWindow });
      return data[fieldName].map((item) => tokensByChannelSchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

export function useTokensByModel(timeWindow?: string, mode: DashboardMode = 'project') {
  const isPersonal = mode === 'personal';
  return useQuery({
    queryKey: ['tokenStatsByModel', timeWindow, isPersonal],
    queryFn: async () => {
      const query = isPersonal ? MY_TOKENS_BY_MODEL_QUERY : TOKENS_BY_MODEL_QUERY;
      const fieldName = isPersonal ? 'myTokenStatsByModel' : 'tokenStatsByModel';
      const data = await graphqlRequest<{ [key: string]: TokensByModel[] }>(query, { timeWindow });
      return data[fieldName].map((item) => tokensByModelSchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

export function useCostByChannel(timeWindow?: string, mode: DashboardMode = 'project') {
  const isPersonal = mode === 'personal';
  return useQuery({
    queryKey: ['costStatsByChannel', timeWindow, isPersonal],
    queryFn: async () => {
      const query = isPersonal ? MY_COST_BY_CHANNEL_QUERY : COST_BY_CHANNEL_QUERY;
      const fieldName = isPersonal ? 'myCostStatsByChannel' : 'costStatsByChannel';
      const data = await graphqlRequest<{ [key: string]: CostByChannel[] }>(query, { timeWindow });
      return data[fieldName].map((item) => costByChannelSchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

export function useCostByModel(timeWindow?: string, mode: DashboardMode = 'project') {
  const isPersonal = mode === 'personal';
  return useQuery({
    queryKey: ['costStatsByModel', timeWindow, isPersonal],
    queryFn: async () => {
      const query = isPersonal ? MY_COST_BY_MODEL_QUERY : COST_BY_MODEL_QUERY;
      const fieldName = isPersonal ? 'myCostStatsByModel' : 'costStatsByModel';
      const data = await graphqlRequest<{ [key: string]: CostByModel[] }>(query, { timeWindow });
      return data[fieldName].map((item) => costByModelSchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

export function useCostByAPIKey(timeWindow?: string, mode: DashboardMode = 'project') {
  const isPersonal = mode === 'personal';
  return useQuery({
    queryKey: ['costStatsByAPIKey', timeWindow, isPersonal],
    queryFn: async () => {
      const query = isPersonal ? MY_COST_BY_API_KEY_QUERY : COST_BY_API_KEY_QUERY;
      const fieldName = isPersonal ? 'myCostStatsByAPIKey' : 'costStatsByAPIKey';
      const data = await graphqlRequest<{ [key: string]: CostByAPIKey[] }>(query, { timeWindow });
      return data[fieldName].map((item) => costByAPIKeySchema.parse(item));
    },
    refetchInterval: 60000,
    placeholderData: (previousData) => previousData,
  });
}

export function useDailyRequestStats(mode: DashboardMode = 'project') {
  const isPersonal = mode === 'personal';
  return useQuery({
    queryKey: ['dailyRequestStats', isPersonal],
    queryFn: async () => {
      const query = isPersonal ? MY_DAILY_REQUEST_STATS_QUERY : DAILY_REQUEST_STATS_QUERY;
      const fieldName = isPersonal ? 'myDailyRequestStats' : 'dailyRequestStats';
      const data = await graphqlRequest<{ [key: string]: DailyRequestStats[] }>(query);
      return data[fieldName].map((item) => dailyRequestStatsSchema.parse(item));
    },
    refetchInterval: 300000,
  });
}

export function useTokenStats(mode: DashboardMode = 'project') {
  const isPersonal = mode === 'personal';
  return useQuery({
    queryKey: ['tokenStats', isPersonal],
    queryFn: async () => {
      const query = isPersonal ? MY_TOKEN_STATS_AGGR_QUERY : TOKEN_STATS_AGGR_QUERY;
      const fieldName = isPersonal ? 'myTokenStats' : 'tokenStats';
      const data = await graphqlRequest<{ [key: string]: TokenStats }>(query);
      return tokenStatsSchema.parse(data[fieldName]);
    },
    refetchInterval: 300000,
  });
}

export function useChannelSuccessRates(limit?: number, timeWindow?: string, mode: DashboardMode = 'project') {
  const isPersonal = mode === 'personal';
  return useQuery({
    queryKey: ['channelSuccessRates', limit, timeWindow, isPersonal],
    queryFn: async () => {
      const query = isPersonal ? MY_CHANNEL_SUCCESS_RATES_QUERY : CHANNEL_SUCCESS_RATES_QUERY;
      const fieldName = isPersonal ? 'myChannelSuccessRates' : 'channelSuccessRates';
      const data = await graphqlRequest<{ [key: string]: ChannelSuccessRate[] }>(
        query,
        { ...(timeWindow != null && { timeWindow }), ...(limit != null && { limit }) }
      );
      return data[fieldName].map((item) => channelSuccessRateSchema.parse(item));
    },
    refetchInterval: 300000,
    placeholderData: (previousData) => previousData,
  });
}

export function useModelPerformanceStats(mode: DashboardMode = 'project') {
  const isPersonal = mode === 'personal';
  return useQuery({
    queryKey: ['modelPerformanceStats', isPersonal],
    queryFn: async () => {
      const query = isPersonal ? MY_MODEL_PERFORMANCE_STATS_QUERY : MODEL_PERFORMANCE_STATS_QUERY;
      const fieldName = isPersonal ? 'myModelPerformanceStats' : 'modelPerformanceStats';
      const data = await graphqlRequest<{ [key: string]: ModelPerformanceStat[] }>(query);
      return data[fieldName].map((item) => modelPerformanceStatSchema.parse(item));
    },
    refetchInterval: 300000,
  });
}

export function useChannelPerformanceStats(mode: DashboardMode = 'project') {
  const isPersonal = mode === 'personal';
  return useQuery({
    queryKey: ['channelPerformanceStats', isPersonal],
    queryFn: async () => {
      const query = isPersonal ? MY_CHANNEL_PERFORMANCE_STATS_QUERY : CHANNEL_PERFORMANCE_STATS_QUERY;
      const fieldName = isPersonal ? 'myChannelPerformanceStats' : 'channelPerformanceStats';
      const data = await graphqlRequest<{ [key: string]: ChannelPerformanceStat[] }>(query);
      return data[fieldName].map((item) => channelPerformanceStatSchema.parse(item));
    },
    refetchInterval: 300000,
  });
}
