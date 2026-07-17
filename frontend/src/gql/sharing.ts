import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { graphqlRequest } from './graphql';

// Share/Unshare Channel Mutations

const SHARE_CHANNEL_MUTATION = `
  mutation ShareChannel($id: ID!, $userIDs: [ID!]!) {
    shareChannel(id: $id, userIDs: $userIDs) {
      id
      ownerID
      visibility
      sharedWith
    }
  }
`;

const UNSHARE_CHANNEL_MUTATION = `
  mutation UnshareChannel($id: ID!, $userIDs: [ID!]!) {
    unshareChannel(id: $id, userIDs: $userIDs) {
      id
      ownerID
      visibility
      sharedWith
    }
  }
`;

export function useShareChannel() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async ({ id, userIDs }: { id: string; userIDs: string[] }) => {
      const data = await graphqlRequest<{
        shareChannel: { id: string; ownerID: string; visibility: string; sharedWith: number[] };
      }>(SHARE_CHANNEL_MUTATION, { id, userIDs });
      return data.shareChannel;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['channels'] });
      toast.success(t('share.messages.shareSuccess'));
    },
    onError: (error) => {
      handleError(error, { context: 'Share Channel' });
    },
  });
}

export function useUnshareChannel() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async ({ id, userIDs }: { id: string; userIDs: string[] }) => {
      const data = await graphqlRequest<{
        unshareChannel: { id: string; ownerID: string; visibility: string; sharedWith: number[] };
      }>(UNSHARE_CHANNEL_MUTATION, { id, userIDs });
      return data.unshareChannel;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['channels'] });
      toast.success(t('share.messages.unshareSuccess'));
    },
    onError: (error) => {
      handleError(error, { context: 'Unshare Channel' });
    },
  });
}

// Publish Request Mutations

const REQUEST_PUBLISH_MUTATION = `
  mutation RequestPublish($resourceType: PublishRequestResourceType!, $resourceID: ID!, $comment: String) {
    requestPublish(resourceType: $resourceType, resourceID: $resourceID, comment: $comment) {
      id
      resourceType
      resourceID
      status
      requestComment
      createdAt
    }
  }
`;

const CANCEL_PUBLISH_REQUEST_MUTATION = `
  mutation CancelPublishRequest($id: ID!) {
    cancelPublishRequest(id: $id)
  }
`;

const REVIEW_PUBLISH_REQUEST_MUTATION = `
  mutation ReviewPublishRequest($id: ID!, $action: ReviewAction!, $comment: String) {
    reviewPublishRequest(id: $id, action: $action, comment: $comment) {
      id
      status
      reviewerID
      reviewComment
    }
  }
`;

export function useRequestPublish() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async ({
      resourceType,
      resourceID,
      comment,
    }: {
      resourceType: 'channel' | 'model';
      resourceID: string;
      comment?: string;
    }) => {
      const data = await graphqlRequest<{
        requestPublish: {
          id: string;
          resourceType: string;
          resourceID: number;
          status: string;
          requestComment: string;
          createdAt: string;
        };
      }>(REQUEST_PUBLISH_MUTATION, { resourceType, resourceID, comment });
      return data.requestPublish;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['publishRequests'] });
      queryClient.invalidateQueries({ queryKey: ['pendingPublishRequests'] });
      toast.success(t('publishRequests.messages.requestSuccess'));
    },
    onError: (error) => {
      handleError(error, { context: 'Request Publish' });
    },
  });
}

export function useCancelPublishRequest() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (id: string) => {
      const data = await graphqlRequest<{ cancelPublishRequest: boolean }>(CANCEL_PUBLISH_REQUEST_MUTATION, { id });
      return data.cancelPublishRequest;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['publishRequests'] });
      queryClient.invalidateQueries({ queryKey: ['pendingPublishRequests'] });
      toast.success(t('publishRequests.messages.cancelSuccess'));
    },
    onError: (error) => {
      handleError(error, { context: 'Cancel Publish Request' });
    },
  });
}

export function useReviewPublishRequest() {
  const queryClient = useQueryClient();
  const { t } = useTranslation();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async ({ id, action, comment }: { id: string; action: 'approve' | 'reject'; comment?: string }) => {
      const data = await graphqlRequest<{
        reviewPublishRequest: {
          id: string;
          status: string;
          reviewerID: string;
          reviewComment: string;
        };
      }>(REVIEW_PUBLISH_REQUEST_MUTATION, { id, action, comment });
      return data.reviewPublishRequest;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({ queryKey: ['publishRequests'] });
      queryClient.invalidateQueries({ queryKey: ['pendingPublishRequests'] });
      const key = variables.action === 'approve' ? 'approveSuccess' : 'rejectSuccess';
      toast.success(t(`publishRequests.messages.${key}`));
    },
    onError: (error) => {
      handleError(error, { context: 'Review Publish Request' });
    },
  });
}

// Publish Requests Query

const PUBLISH_REQUESTS_QUERY = `
  query PublishRequests(
    $first: Int
    $after: Cursor
    $last: Int
    $before: Cursor
    $where: PublishRequestWhereInput
    $orderBy: PublishRequestOrder
  ) {
    publishRequests(first: $first, after: $after, last: $last, before: $before, where: $where, orderBy: $orderBy) {
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
            email
            firstName
            lastName
          }
          reviewer {
            id
            email
            firstName
            lastName
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

export interface PublishRequest {
  id: string;
  createdAt: string;
  updatedAt: string;
  resourceType: 'channel' | 'model';
  resourceID: number;
  requesterID: string;
  status: 'pending' | 'approved' | 'rejected';
  reviewerID: string | null;
  reviewComment: string | null;
  requestComment: string | null;
  requester: {
    id: string;
    email: string;
    firstName: string;
    lastName: string;
  };
  reviewer: {
    id: string;
    email: string;
    firstName: string;
    lastName: string;
  } | null;
}

export interface PublishRequestConnection {
  edges: Array<{ node: PublishRequest; cursor: string }>;
  pageInfo: {
    hasNextPage: boolean;
    hasPreviousPage: boolean;
    startCursor: string;
    endCursor: string;
  };
  totalCount: number;
}

export function usePublishRequests(variables?: {
  first?: number;
  after?: string;
  before?: string;
  last?: number;
  where?: Record<string, unknown>;
  orderBy?: { field: 'CREATED_AT' | 'UPDATED_AT'; direction: 'ASC' | 'DESC' };
}) {
  return useQuery({
    queryKey: ['publishRequests', variables],
    queryFn: async () => {
      const data = await graphqlRequest<{ publishRequests: PublishRequestConnection }>(PUBLISH_REQUESTS_QUERY, variables);
      return data.publishRequests;
    },
  });
}
