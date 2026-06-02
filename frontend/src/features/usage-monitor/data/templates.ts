import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import type { FieldConfig, Variable, DisplayField } from './schema';

const QUOTA_MONITOR_TEMPLATES_QUERY = `
  query QuotaMonitorTemplates {
    quotaMonitorTemplates {
      providerType
      name
      description
      apiUrl
      apiMethod
      headerFormat
      apiBody
      credentialLabel
      credentialPlaceholder
      fields {
        key
        label
        path
        type
        format
        totalPath
        unit
        groupIndex
        displayOrder
      }
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
      }
    }
  }
`;

export type QuotaMonitorTemplate = {
  providerType: string;
  name: string;
  description?: string;
  apiUrl: string;
  apiMethod: 'GET' | 'POST';
  headerFormat: string;
  apiBody?: string;
  credentialLabel?: string;
  credentialPlaceholder?: string;
  fields: FieldConfig[];
  variables: Variable[];
  displayFields: DisplayField[];
};

export function useQuotaMonitorTemplates() {
  const { data } = useQuery({
    queryKey: ['quotaMonitorTemplates'],
    queryFn: async () => {
      const result = await graphqlRequest<{
        quotaMonitorTemplates: QuotaMonitorTemplate[];
      }>(QUOTA_MONITOR_TEMPLATES_QUERY);
      return result.quotaMonitorTemplates ?? [];
    },
    staleTime: Infinity,
  });
  return data ?? [];
}
