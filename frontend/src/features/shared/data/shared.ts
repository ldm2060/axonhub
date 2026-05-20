import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { useErrorHandler } from '@/hooks/use-error-handler';
import { useTranslation } from 'react-i18next';

const MY_SHARED_CHANNELS_QUERY = `
  query MySharedChannels {
    mySharedChannels {
      id
      name
      type
      status
      supportedModels
      tags
      owner {
        id
        firstName
        lastName
        email
      }
      createdAt
    }
  }
`;

const MY_SHARED_MODELS_QUERY = `
  query MySharedModels {
    mySharedModels {
      id
      name
      modelID
      type
      developer
      group
      status
      owner {
        id
        firstName
        lastName
        email
      }
      createdAt
    }
  }
`;

export interface SharedOwner {
  id: string;
  firstName: string;
  lastName: string;
  email: string;
}

export interface SharedChannel {
  id: string;
  name: string;
  type: string;
  status: string;
  supportedModels: string[];
  tags: string[];
  owner: SharedOwner | null;
  createdAt: string;
}

export interface SharedModel {
  id: string;
  name: string;
  modelID: string;
  type: string;
  developer: string;
  group: string;
  status: string;
  owner: SharedOwner | null;
  createdAt: string;
}

export function useMySharedChannels() {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();

  return useQuery({
    queryKey: ['sharedChannels'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ mySharedChannels: SharedChannel[] }>(
          MY_SHARED_CHANNELS_QUERY
        );
        return data.mySharedChannels;
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useMySharedModels() {
  const { handleError } = useErrorHandler();
  const { t } = useTranslation();

  return useQuery({
    queryKey: ['sharedModels'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ mySharedModels: SharedModel[] }>(
          MY_SHARED_MODELS_QUERY
        );
        return data.mySharedModels;
      } catch (error) {
        handleError(error, t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}
