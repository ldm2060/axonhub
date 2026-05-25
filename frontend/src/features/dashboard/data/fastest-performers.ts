import { z } from 'zod';
import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
export type { DashboardMode } from './dashboard';

const REFETCH_INTERVAL_MS = 30000;

export const fastestChannelSchema = z.object({
  channelId: z.string(),
  channelName: z.string(),
  channelType: z.string(),
  throughput: z.number(),
  tokensCount: z.number(),
  latencyMs: z.number(),
  requestCount: z.number(),
});

export const fastestModelSchema = z.object({
  modelId: z.string(),
  modelName: z.string(),
  throughput: z.number(),
  tokensCount: z.number(),
  latencyMs: z.number(),
  requestCount: z.number(),
});

export const fastestChannelsInputSchema = z.object({
  timeWindow: z.string(),
  limit: z.number().optional().default(5),
});

export type FastestChannel = z.infer<typeof fastestChannelSchema>;
export type FastestModel = z.infer<typeof fastestModelSchema>;
export type FastestChannelsInput = z.infer<typeof fastestChannelsInputSchema>;

// Admin queries
const FASTEST_CHANNELS_QUERY = `
  query GetFastestChannels($input: FastestChannelsInput!) {
    fastestChannels(input: $input) {
      channelId
      channelName
      channelType
      throughput
      tokensCount
      latencyMs
      requestCount
    }
  }
`;

const FASTEST_MODELS_QUERY = `
  query GetFastestModels($input: FastestChannelsInput!) {
    fastestModels(input: $input) {
      modelId
      modelName
      throughput
      tokensCount
      latencyMs
      requestCount
    }
  }
`;

// Personal queries
const MY_FASTEST_CHANNELS_QUERY = `
  query GetMyFastestChannels($input: FastestChannelsInput!) {
    myFastestChannels(input: $input) {
      channelId
      channelName
      channelType
      throughput
      tokensCount
      latencyMs
      requestCount
    }
  }
`;

const MY_FASTEST_MODELS_QUERY = `
  query GetMyFastestModels($input: FastestChannelsInput!) {
    myFastestModels(input: $input) {
      modelId
      modelName
      throughput
      tokensCount
      latencyMs
      requestCount
    }
  }
`;

export function useFastestChannels(timeWindow: string = 'day', limit: number = 5, mode: DashboardMode = 'project') {
  const isPersonal = mode === 'personal';
  return useQuery({
    queryKey: ['fastestChannels', timeWindow, limit, isPersonal],
    queryFn: async () => {
      const query = isPersonal ? MY_FASTEST_CHANNELS_QUERY : FASTEST_CHANNELS_QUERY;
      const fieldName = isPersonal ? 'myFastestChannels' : 'fastestChannels';
      const data = await graphqlRequest<{ [key: string]: FastestChannel[] }>(
        query,
        { input: { timeWindow, limit } }
      );
      return data[fieldName].map((item) => fastestChannelSchema.parse(item));
    },
    refetchInterval: REFETCH_INTERVAL_MS,
    placeholderData: (previousData) => previousData,
  });
}

export function useFastestModels(timeWindow: string = 'day', limit: number = 5, mode: DashboardMode = 'project') {
  const isPersonal = mode === 'personal';
  return useQuery({
    queryKey: ['fastestModels', timeWindow, limit, isPersonal],
    queryFn: async () => {
      const query = isPersonal ? MY_FASTEST_MODELS_QUERY : FASTEST_MODELS_QUERY;
      const fieldName = isPersonal ? 'myFastestModels' : 'fastestModels';
      const data = await graphqlRequest<{ [key: string]: FastestModel[] }>(
        query,
        { input: { timeWindow, limit } }
      );
      return data[fieldName].map((item) => fastestModelSchema.parse(item));
    },
    refetchInterval: REFETCH_INTERVAL_MS,
    placeholderData: (previousData) => previousData,
  });
}