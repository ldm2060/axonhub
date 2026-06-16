import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useErrorHandler } from '@/hooks/use-error-handler';
import {
  usageMonitorChannelSchema,
  testResultSchema,
  usageMonitorBindingSummarySchema,
  type FieldConfig,
  type Variable,
  type DisplayField,
  type VariableInput,
  type DisplayFieldInput,
  type UsageMonitorChannel,
  type TestResult,
  type UsageMonitorBindingSummary,
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
      providerType
      apiKey
      channelID
      apiURL
      apiMethod
      apiHeaders
      apiHeadersString
      apiBody
      pollInterval
      fields
      variables {
        key
        path
        type
        groupIndex
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
        group
        groupLabel
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
      variables {
        key
        path
        type
        groupIndex
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
        group
        groupLabel
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
      variables {
        key
        path
        type
        groupIndex
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
        group
        groupLabel
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
        group
        groupLabel
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
      variables {
        key
        path
        type
        groupIndex
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
        group
        groupLabel
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
    providerType: raw.providerType || null,
    apiKey: raw.apiKey ?? null,
    channel: raw.channel ?? null,
    apiUrl: raw.apiURL ?? raw.apiUrl ?? '',
    apiMethod: raw.apiMethod ?? null,
    apiHeaders: raw.apiHeadersString ?? raw.apiHeaders ?? '{}',
    apiBody: raw.apiBody || null,
    pollInterval: raw.pollInterval ?? 300,
    fields: Array.isArray(raw.fields) ? raw.fields : (raw.fields?.items ?? []),
    variables: raw.variables ?? [],
    displayFields: raw.displayFields ?? [],
    status: raw.status ?? 'active',
    lastPollAt: raw.lastPollAt ?? null,
    parsedData: raw.parsedData ?? null,
    lastPollError: raw.lastPollError ?? null,
    createdAt: raw.createdAt ?? '',
    updatedAt: raw.updatedAt ?? '',
  });
}

// Hooks

export function useUsageMonitorChannels() {
  return useQuery({
    queryKey: [USAGE_MONITOR_CHANNELS_KEY],
    queryFn: async () => {
      const data = await graphqlRequest<{
        usageMonitorChannelsList: unknown[];
      }>(USAGE_MONITOR_CHANNELS_QUERY);
      return (data.usageMonitorChannelsList ?? []).map((raw: any) =>
        normalizeChannel(raw),
      );
    },
    refetchInterval: false,
  });
}

export type CreateUsageMonitorChannelInput = {
  name: string;
  source: 'builtin' | 'custom' | 'template';
  channelId?: string;
  providerType?: string;
  apiKey?: string;
  apiUrl: string;
  apiMethod: 'GET' | 'POST';
  apiHeaders: string;
  apiBody?: string;
  pollInterval: number;
  fields: FieldConfig[];
  variables?: VariableInput[];
  displayFields?: DisplayFieldInput[];
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
  variables?: VariableInput[];
  displayFields?: DisplayFieldInput[];
  status?: 'active' | 'paused' | 'error';
  apiKey?: string;
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
  providerType?: string;
  apiKey?: string;
  variables?: VariableInput[];
  displayFields?: DisplayFieldInput[];
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

/**
 * Silent refresh – same mutation as useRefreshUsageMonitorChannel but without
 * toast notifications so it can be used for automatic per-channel polling.
 */
export function useSilentRefreshUsageMonitorChannel() {
  const queryClient = useQueryClient();
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
    },
    onError: (error) => {
      handleError(error, { context: 'Auto-refresh Usage Monitor Channel' });
    },
  });
}

// Usage Monitor Binding Summaries

const USAGE_MONITOR_BINDING_SUMMARIES_QUERY = `
  query UsageMonitorBindingSummaries {
    usageMonitorBindingSummaries {
      channelID
      channelName
      usageMonitorChannelID
      strategy
      enabled
      triggerStatuses
      conditions {
        field
        operator
        value
      }
      matched
      reason
    }
  }
`;

export function useUsageMonitorBindingSummaries(options?: { enabled?: boolean }) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();

  return useQuery({
    queryKey: ['usageMonitorBindingSummaries'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ usageMonitorBindingSummaries: UsageMonitorBindingSummary[] }>(
          USAGE_MONITOR_BINDING_SUMMARIES_QUERY
        );
        return (data.usageMonitorBindingSummaries || []).map((s) => usageMonitorBindingSummarySchema.parse(s));
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: options?.enabled !== false,
  });
}
