import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useErrorHandler } from '@/hooks/use-error-handler';
import {
  usageMonitorChannelSchema,
  testResultSchema,
  type FieldConfig,
  type UsageMonitorChannel,
  type TestResult,
} from './schema';

// Query key
const USAGE_MONITOR_CHANNELS_KEY = 'usageMonitorChannels';

// GraphQL queries and mutations

const USAGE_MONITOR_CHANNELS_QUERY = `
  query UsageMonitorChannelsList {
    usageMonitorChannelsList {
      id
      name
      source
      channelID
      apiURL
      apiMethod
      apiHeaders
      apiHeadersString
      apiBody
      pollInterval
      fields
      status
      lastPollAt
      lastPollError
      createdAt
      updatedAt
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

const CREATE_USAGE_MONITOR_CHANNEL_MUTATION = `
  mutation CreateUsageMonitorChannel($input: CreateUsageMonitorChannelInput!) {
    createUsageMonitorChannel(input: $input) {
      id
      name
      source
      channelID
      apiURL
      apiMethod
      apiHeaders
      apiHeadersString
      apiBody
      pollInterval
      fields
      status
      lastPollAt
      lastPollError
      createdAt
      updatedAt
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

const UPDATE_USAGE_MONITOR_CHANNEL_MUTATION = `
  mutation UpdateUsageMonitorChannel($id: ID!, $input: UpdateUsageMonitorChannelInput!) {
    updateUsageMonitorChannel(id: $id, input: $input) {
      id
      name
      source
      channelID
      apiURL
      apiMethod
      apiHeaders
      apiHeadersString
      apiBody
      pollInterval
      fields
      status
      lastPollAt
      lastPollError
      createdAt
      updatedAt
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

const DELETE_USAGE_MONITOR_CHANNEL_MUTATION = `
  mutation DeleteUsageMonitorChannel($id: ID!) {
    deleteUsageMonitorChannel(id: $id)
  }
`;

const TEST_USAGE_MONITOR_CHANNEL_MUTATION = `
  mutation TestUsageMonitorChannel($input: TestUsageMonitorChannelInput!) {
    testUsageMonitorChannel(input: $input) {
      success
      rawResponse
      parsedFields {
        key
        label
        value
        total
        percent
        unit
        format
        error
      }
      error
    }
  }
`;

const REFRESH_USAGE_MONITOR_CHANNEL_MUTATION = `
  mutation RefreshUsageMonitorChannel($id: ID!) {
    refreshUsageMonitorChannel(id: $id) {
      id
      name
      source
      channelID
      apiURL
      apiMethod
      apiHeaders
      apiHeadersString
      apiBody
      pollInterval
      fields
      status
      lastPollAt
      lastPollError
      createdAt
      updatedAt
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

// Helper to normalize GraphQL field names to camelCase for Zod parsing
function normalizeChannel(raw: any): UsageMonitorChannel {
  return usageMonitorChannelSchema.parse({
    id: raw.id,
    name: raw.name,
    source: raw.source,
    channel: raw.channel ?? null,
    apiUrl: raw.apiURL ?? raw.apiUrl,
    apiMethod: raw.apiMethod,
    apiHeaders: raw.apiHeadersString ?? raw.apiHeaders,
    apiBody: raw.apiBody ?? null,
    pollInterval: raw.pollInterval,
    fields: raw.fields ?? [],
    status: raw.status,
    lastPollAt: raw.lastPollAt ?? null,
    parsedData: raw.parsedData ?? null,
    lastPollError: raw.lastPollError ?? null,
    createdAt: raw.createdAt,
    updatedAt: raw.updatedAt,
  });
}

// Hooks

export function useUsageMonitorChannels() {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();

  return useQuery({
    queryKey: [USAGE_MONITOR_CHANNELS_KEY],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{
          usageMonitorChannelsList: unknown[];
        }>(USAGE_MONITOR_CHANNELS_QUERY);
        return (data.usageMonitorChannelsList ?? []).map((raw: any) =>
          normalizeChannel(raw),
        );
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    refetchInterval: 60_000,
    refetchIntervalInBackground: true,
  });
}

export type CreateUsageMonitorChannelInput = {
  name: string;
  source: 'builtin' | 'custom';
  channelId?: string;
  apiUrl: string;
  apiMethod: 'GET' | 'POST';
  apiHeaders: string;
  apiBody?: string;
  pollInterval: number;
  fields: FieldConfig[];
};

export function useCreateUsageMonitorChannel() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (input: CreateUsageMonitorChannelInput) => {
      const data = await graphqlRequest<{
        createUsageMonitorChannel: any;
      }>(CREATE_USAGE_MONITOR_CHANNEL_MUTATION, { input });
      return normalizeChannel(data.createUsageMonitorChannel);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [USAGE_MONITOR_CHANNELS_KEY] });
      toast.success(t('usageMonitor.messages.createSuccess'));
    },
    onError: (error) => {
      handleError(error, { context: 'Create Usage Monitor Channel' });
    },
  });
}

export type UpdateUsageMonitorChannelInput = {
  name?: string;
  apiUrl?: string;
  apiMethod?: 'GET' | 'POST';
  apiHeaders?: string;
  apiBody?: string;
  pollInterval?: number;
  fields?: FieldConfig[];
  status?: 'active' | 'paused' | 'error';
};

export function useUpdateUsageMonitorChannel() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async ({
      id,
      input,
    }: {
      id: string;
      input: UpdateUsageMonitorChannelInput;
    }) => {
      const data = await graphqlRequest<{
        updateUsageMonitorChannel: any;
      }>(UPDATE_USAGE_MONITOR_CHANNEL_MUTATION, { id, input });
      return normalizeChannel(data.updateUsageMonitorChannel);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [USAGE_MONITOR_CHANNELS_KEY] });
      toast.success(t('usageMonitor.messages.updateSuccess'));
    },
    onError: (error) => {
      handleError(error, { context: 'Update Usage Monitor Channel' });
    },
  });
}

export function useDeleteUsageMonitorChannel() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{
        deleteUsageMonitorChannel: boolean;
      }>(DELETE_USAGE_MONITOR_CHANNEL_MUTATION, { id });
      return data.deleteUsageMonitorChannel;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [USAGE_MONITOR_CHANNELS_KEY] });
      toast.success(t('usageMonitor.messages.deleteSuccess'));
    },
    onError: (error) => {
      handleError(error, { context: 'Delete Usage Monitor Channel' });
    },
  });
}

export type TestUsageMonitorChannelInput = {
  apiUrl: string;
  apiMethod: 'GET' | 'POST';
  apiHeaders: string;
  apiBody?: string;
  fields: FieldConfig[];
};

export function useTestUsageMonitorChannel() {
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (input: TestUsageMonitorChannelInput) => {
      const data = await graphqlRequest<{
        testUsageMonitorChannel: unknown;
      }>(TEST_USAGE_MONITOR_CHANNEL_MUTATION, { input });
      return testResultSchema.parse(data.testUsageMonitorChannel);
    },
    onError: (error) => {
      handleError(error, { context: 'Test Usage Monitor Channel' });
    },
  });
}

export function useRefreshUsageMonitorChannel() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{
        refreshUsageMonitorChannel: any;
      }>(REFRESH_USAGE_MONITOR_CHANNEL_MUTATION, { id });
      return normalizeChannel(data.refreshUsageMonitorChannel);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: [USAGE_MONITOR_CHANNELS_KEY] });
      toast.success(t('usageMonitor.messages.refreshSuccess'));
    },
    onError: (error) => {
      handleError(error, { context: 'Refresh Usage Monitor Channel' });
    },
  });
}
