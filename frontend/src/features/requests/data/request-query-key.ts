interface RequestScopedQueryKeyInput {
  id: string;
  permissions: unknown;
  projectId?: string | null;
  includeAdminFields?: boolean;
}

interface RequestQueryKeyInput extends RequestScopedQueryKeyInput {
  scope?: 'detail' | 'quick-view';
}

export function buildRequestMetadataQueryKey({
  id,
  permissions,
  projectId,
  includeAdminFields,
}: RequestScopedQueryKeyInput) {
  return ['request', 'metadata', id, permissions, projectId, includeAdminFields] as const;
}

export function buildRequestContentQueryKey({
  id,
  permissions,
  projectId,
  includeAdminFields,
  content,
}: RequestScopedQueryKeyInput & { content: 'request' | 'response' }) {
  return ['request', 'content', content, id, permissions, projectId, includeAdminFields] as const;
}

export function buildRequestExecutionContentQueryKey({
  id,
  permissions,
  projectId,
  includeAdminFields,
  executionId,
}: RequestScopedQueryKeyInput & { executionId: string }) {
  return ['request-execution', 'content', executionId, id, permissions, projectId, includeAdminFields] as const;
}

export function buildRequestQueryKey({
  id,
  permissions,
  projectId,
  includeAdminFields,
  scope = 'detail',
}: RequestQueryKeyInput) {
  return ['request', scope, id, permissions, projectId, includeAdminFields] as const;
}
