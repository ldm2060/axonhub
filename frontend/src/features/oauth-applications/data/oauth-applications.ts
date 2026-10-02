import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useErrorHandler } from '@/hooks/use-error-handler';
import {
  oauthApplicationSchema,
  oauthApplicationsConnectionSchema,
  oauthAuthorizationRequestSchema,
  createOAuthApplicationInputSchema,
  updateOAuthApplicationInputSchema,
  type OAuthApplication,
  type OAuthApplicationsConnection,
  type OAuthAuthorizationRequest,
  type CreateOAuthApplicationInput,
  type UpdateOAuthApplicationInput,
} from './schema';

export type {
  OAuthApplication,
  OAuthApplicationsConnection,
  OAuthAuthorizationRequest,
  CreateOAuthApplicationInput,
  UpdateOAuthApplicationInput,
};

const OAUTH_APPLICATION_FIELDS = `
  id
  name
  description
  clientID
  redirectUris
  clientType
  status
  lastUsedAt
  createdAt
  updatedAt
`;

const OAUTH_APPLICATIONS_QUERY = `
  query OAuthApplications(
    $first: Int
    $after: Cursor
    $where: OAuthClientWhereInput
    $orderBy: OAuthClientOrder
  ) {
    oauthClients(first: $first, after: $after, where: $where, orderBy: $orderBy) {
      edges {
        node {
          ${OAUTH_APPLICATION_FIELDS}
        }
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

const CREATE_OAUTH_APPLICATION_MUTATION = `
  mutation CreateOAuthApplication($input: CreateOAuthClientInput!) {
    createOAuthClient(input: $input) {
      client {
        ${OAUTH_APPLICATION_FIELDS}
      }
      clientSecret
    }
  }
`;

const UPDATE_OAUTH_APPLICATION_MUTATION = `
  mutation UpdateOAuthApplication($id: ID!, $input: UpdateOAuthClientInput!) {
    updateOAuthClient(id: $id, input: $input) {
      ${OAUTH_APPLICATION_FIELDS}
    }
  }
`;

const DELETE_OAUTH_APPLICATION_MUTATION = `
  mutation DeleteOAuthApplication($id: ID!) {
    deleteOAuthClient(id: $id)
  }
`;

const ROTATE_OAUTH_APPLICATION_SECRET_MUTATION = `
  mutation RotateOAuthApplicationSecret($id: ID!) {
    rotateOAuthClientSecret(id: $id) {
      client {
        ${OAUTH_APPLICATION_FIELDS}
      }
      clientSecret
    }
  }
`;

const OAUTH_AUTHORIZATION_REQUEST_QUERY = `
  query OAuthAuthorizationRequest($requestId: String!) {
    oauthAuthorizationRequest(requestId: $requestId) {
      clientId
      clientName
      clientDescription
      redirectUriHost
      scopes
      userEmail
    }
  }
`;

const APPROVE_OAUTH_AUTHORIZATION_MUTATION = `
  mutation ApproveOAuthAuthorization($requestId: String!) {
    approveOAuthAuthorization(requestId: $requestId) {
      redirectUrl
    }
  }
`;

const DENY_OAUTH_AUTHORIZATION_MUTATION = `
  mutation DenyOAuthAuthorization($requestId: String!) {
    denyOAuthAuthorization(requestId: $requestId) {
      redirectUrl
    }
  }
`;

export function useOAuthApplications(variables?: Record<string, any>) {
  return useQuery({
    queryKey: ['oauth-applications', variables],
    queryFn: async () => {
      const data = await graphqlRequest<{ oauthClients: OAuthApplicationsConnection }>(OAUTH_APPLICATIONS_QUERY, variables);
      return oauthApplicationsConnectionSchema.parse(data.oauthClients);
    },
  });
}

export function useCreateOAuthApplication() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (input: CreateOAuthApplicationInput) => {
      try {
        const validatedInput = createOAuthApplicationInputSchema.parse(input);
        const data = await graphqlRequest<{ createOAuthClient: { client: OAuthApplication; clientSecret: string } }>(
          CREATE_OAUTH_APPLICATION_MUTATION,
          { input: validatedInput }
        );
        return {
          client: oauthApplicationSchema.parse(data.createOAuthClient.client),
          clientSecret: data.createOAuthClient.clientSecret,
        };
      } catch (error) {
        handleError(error, { context: t('oauthApplications.dialogs.create.title') });
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['oauth-applications'] });
      toast.success(t('common.messages.success'));
    },
  });
}

export function useUpdateOAuthApplication() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async ({ id, input }: { id: string; input: UpdateOAuthApplicationInput }) => {
      try {
        const validatedInput = updateOAuthApplicationInputSchema.parse(input);
        const data = await graphqlRequest<{ updateOAuthClient: OAuthApplication }>(UPDATE_OAUTH_APPLICATION_MUTATION, {
          id,
          input: validatedInput,
        });
        return oauthApplicationSchema.parse(data.updateOAuthClient);
      } catch (error) {
        handleError(error, { context: t('oauthApplications.dialogs.edit.title') });
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['oauth-applications'] });
      toast.success(t('common.messages.success'));
    },
  });
}

export function useDeleteOAuthApplication() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (id: string) => {
      try {
        await graphqlRequest<{ deleteOAuthClient: boolean }>(DELETE_OAUTH_APPLICATION_MUTATION, { id });
      } catch (error) {
        handleError(error, { context: t('oauthApplications.dialogs.delete.title') });
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['oauth-applications'] });
      toast.success(t('common.messages.success'));
    },
  });
}

export function useRotateOAuthApplicationSecret() {
  const { t } = useTranslation();
  const queryClient = useQueryClient();
  const { handleError } = useErrorHandler();

  return useMutation({
    mutationFn: async (id: string) => {
      try {
        const data = await graphqlRequest<{ rotateOAuthClientSecret: { client: OAuthApplication; clientSecret: string } }>(
          ROTATE_OAUTH_APPLICATION_SECRET_MUTATION,
          { id }
        );
        return {
          client: oauthApplicationSchema.parse(data.rotateOAuthClientSecret.client),
          clientSecret: data.rotateOAuthClientSecret.clientSecret,
        };
      } catch (error) {
        handleError(error, { context: t('oauthApplications.dialogs.rotateSecret.title') });
        throw error;
      }
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['oauth-applications'] });
    },
  });
}

export function useOAuthAuthorizationRequest(requestId: string) {
  return useQuery({
    queryKey: ['oauth-authorization-request', requestId],
    queryFn: async () => {
      const data = await graphqlRequest<{ oauthAuthorizationRequest: OAuthAuthorizationRequest }>(OAUTH_AUTHORIZATION_REQUEST_QUERY, {
        requestId,
      });
      return oauthAuthorizationRequestSchema.parse(data.oauthAuthorizationRequest);
    },
    enabled: !!requestId,
    retry: false,
  });
}

export function useApproveOAuthAuthorization() {
  return useMutation({
    mutationFn: async (requestId: string) => {
      const data = await graphqlRequest<{ approveOAuthAuthorization: { redirectUrl: string } }>(APPROVE_OAUTH_AUTHORIZATION_MUTATION, {
        requestId,
      });
      return data.approveOAuthAuthorization.redirectUrl;
    },
  });
}

export function useDenyOAuthAuthorization() {
  return useMutation({
    mutationFn: async (requestId: string) => {
      const data = await graphqlRequest<{ denyOAuthAuthorization: { redirectUrl: string } }>(DENY_OAUTH_AUTHORIZATION_MUTATION, {
        requestId,
      });
      return data.denyOAuthAuthorization.redirectUrl;
    },
  });
}
