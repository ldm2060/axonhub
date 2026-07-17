import { z } from 'zod';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useErrorHandler } from '@/hooks/use-error-handler';

// Schema definitions
export const publishRequestSchema = z.object({
  id: z.string(),
  createdAt: z.string(),
  updatedAt: z.string(),
  resourceType: z.enum(['channel', 'model']),
  resourceID: z.number(),
  requesterID: z.string(),
  status: z.enum(['pending', 'approved', 'rejected']),
  reviewerID: z.string().nullable(),
  reviewComment: z.string().nullable(),
  requestComment: z.string().nullable(),
  requester: z.object({
    id: z.string(),
    firstName: z.string(),
    lastName: z.string(),
    email: z.string(),
  }),
  reviewer: z
    .object({
      id: z.string(),
      firstName: z.string(),
      lastName: z.string(),
      email: z.string(),
    })
    .nullable(),
});

export type PublishRequest = z.infer<typeof publishRequestSchema>;

const PENDING_PUBLISH_REQUESTS_QUERY = `
  query GetPendingPublishRequests {
    publishRequests(
      first: 50,
      where: { status: pending },
      orderBy: { field: CREATED_AT, direction: DESC }
    ) {
      edges {
        node {
          id
          createdAt
          updatedAt
          resourceType
          resourceID
          requesterID
          status
          reviewerID
          reviewComment
          requestComment
          requester {
            id
            firstName
            lastName
            email
          }
          reviewer {
            id
            firstName
            lastName
            email
          }
        }
      }
    }
  }
`;

const REVIEW_PUBLISH_REQUEST_MUTATION = `
  mutation ReviewPublishRequest($id: ID!, $action: ReviewAction!, $comment: String) {
    reviewPublishRequest(id: $id, action: $action, comment: $comment) {
      id
      status
    }
  }
`;

export function usePendingPublishRequests(options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: ['pendingPublishRequests'],
    queryFn: async () => {
      const data = await graphqlRequest<{
        publishRequests: {
          edges: Array<{ node: unknown }>;
        };
      }>(PENDING_PUBLISH_REQUESTS_QUERY);
      return (data.publishRequests?.edges || []).map((edge) => publishRequestSchema.parse(edge.node));
    },
    refetchInterval: 30000,
    enabled: options?.enabled !== false,
  });
}

export function useReviewPublishRequest() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async ({ id, action, comment }: { id: string; action: 'approve' | 'reject'; comment?: string }) => {
      try {
        const data = await graphqlRequest<{
          reviewPublishRequest: { id: string; status: string };
        }>(REVIEW_PUBLISH_REQUEST_MUTATION, {
          id,
          action,
          comment: comment || null,
        });
        return data.reviewPublishRequest;
      } catch (error) {
        handleError(error, { context: 'Review Publish Request' });
        throw error;
      }
    },
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['pendingPublishRequests'] });
      const messageKey = variables.action === 'approve' ? 'publish.approveSuccess' : 'publish.rejectSuccess';
      toast.success(t(messageKey));
    },
  });
}
