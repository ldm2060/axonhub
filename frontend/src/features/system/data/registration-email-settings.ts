import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { toast } from 'sonner';
import i18n from '@/lib/i18n';
import { useErrorHandler } from '@/hooks/use-error-handler';

// Registration Settings
const REGISTRATION_SETTINGS_QUERY = `
  query RegistrationSettings {
    registrationSettings {
      enabled
      mode
      defaultScopes
    }
  }
`;

const UPDATE_REGISTRATION_SETTINGS_MUTATION = `
  mutation UpdateRegistrationSettings($input: UpdateRegistrationSettingsInput!) {
    updateRegistrationSettings(input: $input)
  }
`;

export interface RegistrationSettings {
  enabled: boolean;
  mode: string;
  defaultScopes: string[];
}

export interface UpdateRegistrationSettingsInput {
  enabled?: boolean;
  mode?: string;
  defaultScopes?: string[];
}

export function useRegistrationSettings() {
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['registrationSettings'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ registrationSettings: RegistrationSettings }>(REGISTRATION_SETTINGS_QUERY);
        return data.registrationSettings;
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useUpdateRegistrationSettings() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: UpdateRegistrationSettingsInput) => {
      const data = await graphqlRequest<{ updateRegistrationSettings: boolean }>(UPDATE_REGISTRATION_SETTINGS_MUTATION, { input });
      return data.updateRegistrationSettings;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['registrationSettings'] });
      toast.success(i18n.t('common.success.systemUpdated'));
    },
    onError: () => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'));
    },
  });
}

// Email Settings
const EMAIL_SETTINGS_QUERY = `
  query EmailSettings {
    emailSettings {
      smtpHost
      smtpPort
      smtpUsername
      smtpPassword
      encryption
      fromName
      fromAddress
      connected
    }
  }
`;

const UPDATE_EMAIL_SETTINGS_MUTATION = `
  mutation UpdateEmailSettings($input: UpdateEmailSettingsInput!) {
    updateEmailSettings(input: $input)
  }
`;

const TEST_EMAIL_CONNECTION_MUTATION = `
  mutation TestEmailConnection {
    testEmailConnection
  }
`;

export interface EmailSettings {
  smtpHost: string;
  smtpPort: number;
  smtpUsername: string;
  smtpPassword: string;
  encryption: string;
  fromName: string;
  fromAddress: string;
  connected: boolean;
}

export interface UpdateEmailSettingsInput {
  smtpHost?: string;
  smtpPort?: number;
  smtpUsername?: string;
  smtpPassword?: string;
  encryption?: string;
  fromName?: string;
  fromAddress?: string;
}

export function useEmailSettings() {
  const { handleError } = useErrorHandler();

  return useQuery({
    queryKey: ['emailSettings'],
    queryFn: async () => {
      try {
        const data = await graphqlRequest<{ emailSettings: EmailSettings }>(EMAIL_SETTINGS_QUERY);
        return data.emailSettings;
      } catch (error) {
        handleError(error, i18n.t('common.errors.internalServerError'));
        throw error;
      }
    },
  });
}

export function useUpdateEmailSettings() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: UpdateEmailSettingsInput) => {
      const data = await graphqlRequest<{ updateEmailSettings: boolean }>(UPDATE_EMAIL_SETTINGS_MUTATION, { input });
      return data.updateEmailSettings;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['emailSettings'] });
      toast.success(i18n.t('common.success.systemUpdated'));
    },
    onError: () => {
      toast.error(i18n.t('common.errors.systemUpdateFailed'));
    },
  });
}

export function useTestEmailConnection() {
  return useMutation({
    mutationFn: async () => {
      const data = await graphqlRequest<{ testEmailConnection: boolean }>(TEST_EMAIL_CONNECTION_MUTATION);
      return data.testEmailConnection;
    },
    onSuccess: () => {
      toast.success(i18n.t('system.email.testSuccess'));
    },
    onError: () => {
      toast.error(i18n.t('system.email.testFailed'));
    },
  });
}
