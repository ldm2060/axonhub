interface RequestQueryKeyInput {
  id: string;
  permissions: unknown;
  projectId?: string | null;
  includeAdminFields?: boolean;
  scope?: 'detail' | 'quick-view';
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
