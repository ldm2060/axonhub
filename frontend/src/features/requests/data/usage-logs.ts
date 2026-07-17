import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useSelectedProjectId } from '@/stores/projectStore';
import { useUsageLogPermissions } from '../../../gql/useUsageLogPermissions';
import { UsageLog, UsageLogConnection, usageLogConnectionSchema, usageLogSchema } from './usage-logs-schema';

// Dynamic GraphQL query builder
function buildUsageLogsQuery(permissions: { canViewChannels: boolean }) {
  const channelFields = permissions.canViewChannels
    ? `
          channel {
            id
            name
            type
          }`
    : '';

  return `
    query GetUsageLogs($first: Int, $after: Cursor, $orderBy: UsageLogOrder, $where: UsageLogWhereInput) {
      usageLogs(first: $first, after: $after, orderBy: $orderBy, where: $where) {
        edges {
          node {
            id
            createdAt
            updatedAt
            requestID${channelFields}
            modelID
            promptTokens
            completionTokens
            totalTokens
            promptAudioTokens
            promptCachedTokens
            promptWriteCachedTokens
            completionAudioTokens
            completionReasoningTokens
            completionAcceptedPredictionTokens
            completionRejectedPredictionTokens
            source
            format
            totalCost
            costItems {
              itemCode
              quantity
              subtotal
            }
          }
          cursor
        }
        pageInfo {
          hasNextPage
          hasPreviousPage
          startCursor
          endCursor
        }
        totalCount
      }
    }
  `;
}

function buildUsageLogDetailQuery(permissions: { canViewChannels: boolean }) {
  const channelFields = permissions.canViewChannels
    ? `
        channel {
          id
          name
          type
        }`
    : '';

  return `
    query GetUsageLog($id: ID!) {
      node(id: $id) {
        ... on UsageLog {
          id
          createdAt
          updatedAt
          requestID${channelFields}
          modelID
          promptTokens
          completionTokens
          totalTokens
          promptAudioTokens
          promptCachedTokens
          promptWriteCachedTokens
          completionAudioTokens
          completionReasoningTokens
          completionAcceptedPredictionTokens
          completionRejectedPredictionTokens
          source
          format
          totalCost
          costItems {
            itemCode
            quantity
            subtotal
          }
        }
      }
    }
  `;
}

// Query hooks
export function useUsageLogs(
  variables?: {
    first?: number;
    after?: string;
    orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' };
    where?: {
      source?: string;
      modelID?: string;
      channelID?: string;
      projectID?: string;
      requestID?: string;
      [key: string]: any;
    };
  },
  options?: { projectId?: string | null; enabled?: boolean }
) {
  const permissions = useUsageLogPermissions();
  const selectedProjectId = useSelectedProjectId();
  const projectId = options?.projectId !== undefined ? options.projectId : selectedProjectId;
  const enabled = options?.enabled ?? !!projectId;

  return useQuery({
    queryKey: ['usageLogs', variables, permissions, projectId],
    queryFn: async () => {
      const query = buildUsageLogsQuery(permissions);
      const headers = projectId ? { 'X-Project-ID': projectId } : undefined;
      const data = await graphqlRequest<{ usageLogs: UsageLogConnection }>(query, variables, headers);
      return usageLogConnectionSchema.parse(data?.usageLogs);
    },
    enabled,
  });
}

export function useUsageLog(id: string) {
  const permissions = useUsageLogPermissions();
  const selectedProjectId = useSelectedProjectId();

  return useQuery({
    queryKey: ['usageLog', id, permissions, selectedProjectId],
    queryFn: async () => {
      const query = buildUsageLogDetailQuery(permissions);
      const headers = selectedProjectId ? { 'X-Project-ID': selectedProjectId } : undefined;
      const data = await graphqlRequest<{ node: UsageLog }>(query, { id }, headers);
      if (!data.node) {
        throw new Error('Usage log not found');
      }
      return usageLogSchema.parse(data.node);
    },
    enabled: !!id,
  });
}
