import { useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useTranslation } from 'react-i18next';
import { useSelectedProjectId } from '@/stores/projectStore';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { useRequestPermissions } from '../../../hooks/useRequestPermissions';
import {
  buildRequestContentQueryKey,
  buildRequestExecutionContentQueryKey,
  buildRequestMetadataQueryKey,
  buildRequestQueryKey,
} from './request-query-key';
import {
  Request,
  RequestConnection,
  RequestContent,
  RequestExecutionContent,
  RequestExecutionSummaryConnection,
  RequestMetadata,
  requestConnectionSchema,
  requestContentSchema,
  requestExecutionContentSchema,
  requestExecutionSummaryConnectionSchema,
  requestMetadataSchema,
  requestSchema,
} from './schema';

interface RequestQueryOptions {
  includeAdminFields?: boolean;
}

// Dynamic GraphQL query builder
function buildRequestsQuery(
  permissions: { canViewApiKeys: boolean; canViewChannels: boolean; canViewProjects: boolean; canViewUsers: boolean },
  options: RequestQueryOptions = {}
) {
  const adminProjectFields =
    options.includeAdminFields && permissions.canViewProjects
      ? `
            project {
              id
              name
            }`
      : '';

  const apiKeyUserFields =
    options.includeAdminFields && permissions.canViewUsers
      ? `
            user {
              id
              email
              firstName
              lastName
            }`
      : '';

  const apiKeyFields = permissions.canViewApiKeys
    ? `
          apiKey {
            id
            name${apiKeyUserFields}
          }`
    : '';

  const requestChannelFields = permissions.canViewChannels
    ? `
                channel {
                  id
                  name
                }`
    : '';

  const executionChannelFields = permissions.canViewChannels
    ? `
                  channel {
                    id
                    name
                  }`
    : '';

  return `
    query GetRequests(
      $first: Int
      $after: Cursor
      $last: Int
      $before: Cursor
      $orderBy: RequestOrder
      $where: RequestWhereInput
    ) {
      requests(first: $first, after: $after, last: $last, before: $before, orderBy: $orderBy, where: $where) {
        edges {
          node {
            id
            createdAt
            updatedAt${adminProjectFields}${apiKeyFields}${requestChannelFields}
            source
            modelID
            format
            reasoningEffort
            stream
            status
            clientIP
            metricsLatencyMs
            metricsFirstTokenLatencyMs
            metricsReasoningDurationMs
            executions(first: 10, orderBy: { field: CREATED_AT, direction: DESC }) {
              edges {
                node {
                  modelID
                  reasoningEffort
                  status
                  reasoningEffort
                  passThroughApplied${executionChannelFields}
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
            usageLogs(first: 1) {
              edges {
                node {
                  id
                  promptTokens
                  completionTokens
                  completionReasoningTokens
                  totalTokens
                  promptCachedTokens
                  promptWriteCachedTokens
                  totalCost
                }
              }
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

function buildRequestDetailQuery(
  permissions: { canViewApiKeys: boolean; canViewChannels: boolean; canViewProjects: boolean; canViewUsers: boolean },
  options: RequestQueryOptions = {}
) {
  const adminProjectFields =
    options.includeAdminFields && permissions.canViewProjects
      ? `
            project {
              id
              name
            }`
      : '';

  const apiKeyUserFields =
    options.includeAdminFields && permissions.canViewUsers
      ? `
            user {
              id
              email
              firstName
              lastName
            }`
      : '';

  const apiKeyFields = permissions.canViewApiKeys
    ? `
          apiKey {
            id
            name${apiKeyUserFields}
        }`
    : '';

  const requestChannelFields = permissions.canViewChannels
    ? `
          channel {
            id
            name
          }`
    : '';

  return `
    query GetRequestDetail($id: ID!) {
      node(id: $id) {
        ... on Request {
          id
          createdAt
          updatedAt${adminProjectFields}${apiKeyFields}${requestChannelFields}
          source
          modelID
          stream
          clientIP
          projectID
          dataStorageID
          contentSaved
          contentStorageKey
          requestHeaders
          requestBody
          responseBody
          responseChunks
          status
          format
          metricsReasoningDurationMs
          usageLogs(first: 1) {
            edges {
              node {
                  id
                  promptTokens
                  completionTokens
                  completionReasoningTokens
                  totalTokens
                  promptCachedTokens
                  promptWriteCachedTokens
                  totalCost
                }
            }
          }
        }
      }
    }
  `;
}

function buildRequestMetadataQuery(
  permissions: { canViewApiKeys: boolean; canViewChannels: boolean; canViewProjects: boolean; canViewUsers: boolean },
  options: RequestQueryOptions = {}
) {
  const adminProjectFields =
    options.includeAdminFields && permissions.canViewProjects
      ? `
            project {
              id
              name
            }`
      : '';

  const apiKeyUserFields =
    options.includeAdminFields && permissions.canViewUsers
      ? `
            user {
              id
              email
              firstName
              lastName
            }`
      : '';

  const apiKeyFields = permissions.canViewApiKeys
    ? `
          apiKey {
            id
            name${apiKeyUserFields}
        }`
    : '';

  const requestChannelFields = permissions.canViewChannels
    ? `
          channel {
            id
            name
          }`
    : '';

  return `
    query GetRequestMetadata($id: ID!) {
      node(id: $id) {
        ... on Request {
          id
          createdAt
          updatedAt${adminProjectFields}${apiKeyFields}${requestChannelFields}
          source
          modelID
          stream
          clientIP
          projectID
          dataStorageID
          contentSaved
          contentStorageKey
          status
          format
          metricsReasoningDurationMs
          usageLogs(first: 1) {
            edges {
              node {
                id
                promptTokens
                completionTokens
                completionReasoningTokens
                totalTokens
                promptCachedTokens
                promptWriteCachedTokens
                totalCost
              }
            }
          }
        }
      }
    }
  `;
}

function buildRequestContentQuery() {
  return `
    query GetRequestContent($id: ID!) {
      node(id: $id) {
        ... on Request {
          id
          requestHeaders
          requestBody
        }
      }
    }
  `;
}

function buildResponseContentQuery() {
  return `
    query GetResponseContent($id: ID!) {
      node(id: $id) {
        ... on Request {
          id
          responseBody
          responseChunks
        }
      }
    }
  `;
}

function buildRequestExecutionSummariesQuery(permissions: { canViewChannels: boolean }) {
  const channelFields = permissions.canViewChannels
    ? `
              channel {
                  id
                  name
                  type
                  baseURL
              }`
    : '';

  return `
    query GetRequestExecutions(
      $requestID: ID!
      $first: Int
      $after: Cursor
      $orderBy: RequestExecutionOrder
      $where: RequestExecutionWhereInput
    ) {
      node(id: $requestID) {
        ... on Request {
          executions(first: $first, after: $after, orderBy: $orderBy, where: $where) {
            edges {
              node {
                id
                createdAt
                updatedAt
                requestID${channelFields}
                modelID
                projectID
                dataStorageID
                errorMessage
                responseStatusCode
                status
                format
                reasoningEffort
                stream
                requestURL
                passThroughApplied
                metricsFirstTokenLatencyMs
                metricsReasoningDurationMs
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
      }
    }
  `;
}

function buildRequestExecutionContentQuery(permissions: { canViewChannels: boolean }) {
  const channelFields = permissions.canViewChannels
    ? `
          channel {
            id
            name
            type
            baseURL
          }`
    : '';

  return `
    query GetRequestExecutionContent($id: ID!) {
      node(id: $id) {
        ... on RequestExecution {
          id${channelFields}
          format
          requestURL
          requestHeaders
          requestBody
          responseBody
          responseChunks
        }
      }
    }
  `;
}

// Query hooks
export function useRequests(
  variables?: {
    first?: number;
    after?: string;
    last?: number;
    before?: string;
    orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' };
    where?: {
      status?: string;
      source?: string;
      channelID?: string;
      channelIDIn?: string[];
      statusIn?: string[];
      sourceIn?: string[];
      projectID?: string;
      [key: string]: any;
    };
  },
  options?: { projectId?: string | null; scopeToSelectedProject?: boolean; enabled?: boolean; includeAdminFields?: boolean }
) {
  const permissions = useRequestPermissions({ systemOnly: options?.projectId === null });
  const selectedProjectId = useSelectedProjectId();
  const scopeToSelectedProject = options?.scopeToSelectedProject ?? true;
  const projectId = options?.projectId !== undefined ? options.projectId : selectedProjectId;
  const enabled = options?.enabled ?? true;

  return useQuery({
    queryKey: ['requests', variables, permissions, projectId, scopeToSelectedProject, options?.includeAdminFields],
    queryFn: async () => {
      const query = buildRequestsQuery(permissions, { includeAdminFields: options?.includeAdminFields });
      const headers = projectId ? { 'X-Project-ID': projectId } : undefined;

      // Add project filter if project scoping is enabled
      const finalVariables = {
        ...variables,
        where: {
          ...variables?.where,
          ...(scopeToSelectedProject && projectId && { projectID: projectId }),
        },
      };

      const data = await graphqlRequest<{ requests: RequestConnection }>(query, finalVariables, headers);
      return requestConnectionSchema.parse(data?.requests);
    },
    enabled,
    refetchOnWindowFocus: false,
  });
}

export function useRequestMetadata(
  id: string,
  options?: {
    projectId?: string | null;
    enabled?: boolean;
    disableAutoRefresh?: boolean;
    includeAdminFields?: boolean;
  }
) {
  const permissions = useRequestPermissions({ systemOnly: options?.projectId === null });
  const selectedProjectId = useSelectedProjectId();
  const projectId = options?.projectId !== undefined ? options.projectId : selectedProjectId;
  const queryKey = buildRequestMetadataQueryKey({
    id,
    permissions,
    projectId,
    includeAdminFields: options?.includeAdminFields,
  });

  return useQuery<RequestMetadata>({
    queryKey: [...queryKey, id, permissions, projectId, options?.includeAdminFields],
    queryFn: async ({ signal }) => {
      const headers = projectId ? { 'X-Project-ID': projectId } : undefined;
      const data = await graphqlRequest<{ node: RequestMetadata }>(
        buildRequestMetadataQuery(permissions, { includeAdminFields: options?.includeAdminFields }),
        { id },
        headers,
        signal
      );
      if (!data.node) throw new Error('Request not found');
      return requestMetadataSchema.parse(data.node);
    },
    enabled: (options?.enabled ?? true) && !!id,
    refetchInterval: (query) => {
      if (options?.disableAutoRefresh) return false;
      return query.state.data?.status === 'processing' ? 2000 : false;
    },
  });
}

export function useRequestContent(
  id: string,
  options: {
    kind: 'request' | 'response';
    projectId?: string | null;
    enabled?: boolean;
    includeAdminFields?: boolean;
  }
) {
  const permissions = useRequestPermissions({ systemOnly: options.projectId === null });
  const selectedProjectId = useSelectedProjectId();
  const projectId = options.projectId !== undefined ? options.projectId : selectedProjectId;
  const queryKey = buildRequestContentQueryKey({
    id,
    permissions,
    projectId,
    includeAdminFields: options.includeAdminFields,
    content: options.kind,
  });

  return useQuery<RequestContent>({
    queryKey: [...queryKey, id, permissions, projectId, options?.includeAdminFields, options.kind],
    queryFn: async ({ signal }) => {
      const headers = projectId ? { 'X-Project-ID': projectId } : undefined;
      const query = options.kind === 'request' ? buildRequestContentQuery() : buildResponseContentQuery();
      const data = await graphqlRequest<{ node: RequestContent }>(query, { id }, headers, signal);
      if (!data.node) throw new Error('Request not found');
      return requestContentSchema.parse(data.node);
    },
    enabled: (options.enabled ?? true) && !!id,
    gcTime: 0,
  });
}

export function useRequest(
  id: string,
  options?: {
    projectId?: string | null;
    enabled?: boolean;
    disableAutoRefresh?: boolean;
    includeAdminFields?: boolean;
    gcTime?: number;
    queryScope?: 'detail' | 'quick-view';
  }
) {
  const permissions = useRequestPermissions({ systemOnly: options?.projectId === null });
  const selectedProjectId = useSelectedProjectId();
  const queryClient = useQueryClient();
  const projectId = options?.projectId !== undefined ? options.projectId : selectedProjectId;
  const enabled = options?.enabled ?? true;

  const queryKey = buildRequestQueryKey({
    id,
    permissions,
    projectId,
    includeAdminFields: options?.includeAdminFields,
    scope: options?.queryScope,
  });

  return useQuery({
    queryKey: [...queryKey, id, permissions, projectId, options?.includeAdminFields, queryClient],
    queryFn: async () => {
      const headers = projectId ? { 'X-Project-ID': projectId } : undefined;
      const previousRequest = queryClient.getQueryData<Request>(queryKey);
      const shouldUseLightweightPolling = previousRequest?.status === 'processing';

      const query = shouldUseLightweightPolling
        ? buildRequestMetadataQuery(permissions, { includeAdminFields: options?.includeAdminFields })
        : buildRequestDetailQuery(permissions, { includeAdminFields: options?.includeAdminFields });

      const data = await graphqlRequest<{ node: Request }>(query, { id }, headers);
      if (!data.node) {
        throw new Error('Request not found');
      }

      const parsedRequest = requestSchema.parse(data.node);

      if (!shouldUseLightweightPolling) {
        return parsedRequest;
      }

      if (parsedRequest.status !== 'processing') {
        const fullData = await graphqlRequest<{ node: Request }>(
          buildRequestDetailQuery(permissions, { includeAdminFields: options?.includeAdminFields }),
          { id },
          headers
        );
        if (!fullData.node) {
          throw new Error('Request not found');
        }
        return requestSchema.parse(fullData.node);
      }

      return requestSchema.parse({
        ...previousRequest,
        ...parsedRequest,
        requestHeaders: previousRequest?.requestHeaders,
        requestBody: previousRequest?.requestBody,
        responseBody: previousRequest?.responseBody,
        responseChunks: previousRequest?.responseChunks,
        usageLogs: previousRequest?.usageLogs,
      });
    },
    enabled: enabled && !!id,
    gcTime: options?.gcTime,
    refetchInterval: (query) => {
      if (options?.disableAutoRefresh) {
        return false;
      }

      return query.state.data?.status === 'processing' ? 2000 : false;
    },
  });
}

/**
 * Imperative (non-hook) fetch of a page of requests for drawer navigation.
 * direction 'older' fetches the page after endCursor (older in DESC order).
 * direction 'newer' fetches the page before startCursor (newer in DESC order).
 */
export async function fetchAdjacentRequestPage(params: {
  cursor: string;
  direction: 'older' | 'newer';
  pageSize: number;
  where?: Record<string, any>;
  permissions: { canViewApiKeys: boolean; canViewChannels: boolean; canViewProjects: boolean; canViewUsers: boolean };
  projectId?: string | null;
  includeAdminFields?: boolean;
}): Promise<{ requests: Request[]; pageInfo: RequestConnection['pageInfo'] }> {
  const query = buildRequestsQuery(params.permissions, { includeAdminFields: params.includeAdminFields });
  const variables =
    params.direction === 'older' ? { first: params.pageSize, after: params.cursor } : { last: params.pageSize, before: params.cursor };

  const where: Record<string, any> = { ...params.where };
  if (params.projectId) where.projectID = params.projectId;

  const headers = params.projectId ? { 'X-Project-ID': params.projectId } : undefined;
  const data = await graphqlRequest<{ requests: RequestConnection }>(
    query,
    { ...variables, where: Object.keys(where).length > 0 ? where : undefined, orderBy: { field: 'CREATED_AT', direction: 'DESC' } },
    headers
  );
  const result = requestConnectionSchema.parse(data?.requests);
  return { requests: result.edges.map((e) => e.node), pageInfo: result.pageInfo };
}

export function useRequestExecutions(
  requestID: string,
  variables?: {
    first?: number;
    after?: string;
    orderBy?: { field: 'CREATED_AT'; direction: 'ASC' | 'DESC' };
    where?: Record<string, any>;
  },
  options?: { projectId?: string | null; enabled?: boolean }
) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();
  const permissions = useRequestPermissions({ systemOnly: options?.projectId === null });
  const selectedProjectId = useSelectedProjectId();
  const projectId = options?.projectId !== undefined ? options.projectId : selectedProjectId;

  return useQuery({
    queryKey: ['request-executions', requestID, variables, permissions, projectId, handleError, t],
    queryFn: async ({ signal }) => {
      try {
        const query = buildRequestExecutionSummariesQuery(permissions);
        const headers = projectId ? { 'X-Project-ID': projectId } : undefined;
        const finalVariables = {
          requestID,
          ...variables,
        };
        const data = await graphqlRequest<{ node: { executions: RequestExecutionSummaryConnection } }>(
          query,
          finalVariables,
          headers,
          signal
        );
        return requestExecutionSummaryConnectionSchema.parse(data?.node?.executions);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: (options?.enabled ?? true) && !!requestID,
  });
}

export function useRequestExecutionContent(
  requestID: string,
  executionID: string,
  options?: { projectId?: string | null; enabled?: boolean; includeAdminFields?: boolean }
) {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();
  const permissions = useRequestPermissions({ systemOnly: options?.projectId === null });
  const selectedProjectId = useSelectedProjectId();
  const projectId = options?.projectId !== undefined ? options.projectId : selectedProjectId;
  const queryKey = buildRequestExecutionContentQueryKey({
    id: requestID,
    executionId: executionID,
    permissions,
    projectId,
    includeAdminFields: options?.includeAdminFields,
  });

  return useQuery<RequestExecutionContent>({
    queryKey: [...queryKey, projectId, permissions, executionID, handleError, t],
    queryFn: async ({ signal }) => {
      try {
        const headers = projectId ? { 'X-Project-ID': projectId } : undefined;
        const data = await graphqlRequest<{ node: RequestExecutionContent }>(
          buildRequestExecutionContentQuery(permissions),
          { id: executionID },
          headers,
          signal
        );
        if (!data.node) throw new Error('Request execution not found');
        return requestExecutionContentSchema.parse(data.node);
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
    enabled: (options?.enabled ?? true) && !!requestID && !!executionID,
    gcTime: 0,
  });
}
