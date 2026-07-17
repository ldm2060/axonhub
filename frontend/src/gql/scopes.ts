import { z } from 'zod';
import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from './graphql';

// Scope Info schema
export const scopeInfoSchema = z.object({
  scope: z.string(),
  description: z.string(),
  levels: z.array(z.string()),
});
export type ScopeInfo = z.infer<typeof scopeInfoSchema>;

const ALL_SCOPES_QUERY = `
  query GetAllScopes($level: String) {
    allScopes(level: $level) {
      scope
      description
      levels
    }
  }
`;

export function useAllScopes(level?: 'system' | 'project') {
  return useQuery({
    queryKey: ['allScopes', level],
    queryFn: async () => {
      const data = await graphqlRequest<{ allScopes: ScopeInfo[] }>(ALL_SCOPES_QUERY, { level });
      return data.allScopes.map((scope) => scopeInfoSchema.parse(scope));
    },
  });
}
