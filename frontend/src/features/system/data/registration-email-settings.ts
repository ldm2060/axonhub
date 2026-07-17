import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { toast } from 'sonner';
import i18n from '@/lib/i18n';

// Registration Settings
const REGISTRATION_SETTINGS_QUERY = `
  query RegistrationSettings {
    registrationSettings {
      allowSignUp
      approvalRequired
      defaultUserScopes
      emailAllowPatterns
      emailDenyPatterns
    }
  }
`;

const UPDATE_REGISTRATION_SETTINGS_MUTATION = `
  mutation UpdateRegistrationSettings($input: UpdateRegistrationSettingsInput!) {
    updateRegistrationSettings(input: $input) {
      allowSignUp
      approvalRequired
      defaultUserScopes
      emailAllowPatterns
      emailDenyPatterns
    }
  }
`;

export interface RegistrationSettings {
  allowSignUp: boolean;
  approvalRequired: boolean;
  defaultUserScopes: string[];
  emailAllowPatterns: string[];
  emailDenyPatterns: string[];
}

export interface UpdateRegistrationSettingsInput {
  allowSignUp?: boolean;
  approvalRequired?: boolean;
  defaultUserScopes?: string[];
  emailAllowPatterns?: string[];
  emailDenyPatterns?: string[];
}

export function useRegistrationSettings() {
  return useQuery({
    queryKey: ['registrationSettings'],
    queryFn: async () => {
      const data = await graphqlRequest<{ registrationSettings: RegistrationSettings }>(REGISTRATION_SETTINGS_QUERY);
      return data.registrationSettings;
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
      smtpUser
      smtpPassword
      encryption
      skipTLSVerify
      fromName
      fromAddress
    }
  }
`;

const UPDATE_EMAIL_SETTINGS_MUTATION = `
  mutation UpdateEmailSettings($input: UpdateEmailSettingsInput!) {
    updateEmailSettings(input: $input) {
      smtpHost
      smtpPort
      smtpUser
      smtpPassword
      encryption
      skipTLSVerify
      fromName
      fromAddress
    }
  }
`;

const TEST_EMAIL_CONNECTION_MUTATION = `
  mutation TestEmailConnection {
    testEmailConnection {
      success
      message
    }
  }
`;

export interface EmailSettings {
  smtpHost: string;
  smtpPort: number;
  smtpUser: string;
  smtpPassword: string;
  encryption: string;
  skipTLSVerify: boolean;
  fromName: string;
  fromAddress: string;
}

export interface UpdateEmailSettingsInput {
  smtpHost?: string;
  smtpPort?: number;
  smtpUser?: string;
  smtpPassword?: string;
  encryption?: string;
  skipTLSVerify?: boolean;
  fromName?: string;
  fromAddress?: string;
}

export function useEmailSettings() {
  return useQuery({
    queryKey: ['emailSettings'],
    queryFn: async () => {
      const data = await graphqlRequest<{ emailSettings: EmailSettings }>(EMAIL_SETTINGS_QUERY);
      return data.emailSettings;
    },
  });
}

export function useUpdateEmailSettings() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: async (input: UpdateEmailSettingsInput) => {
      const data = await graphqlRequest<{ updateEmailSettings: EmailSettings }>(UPDATE_EMAIL_SETTINGS_MUTATION, { input });
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
      const data = await graphqlRequest<{ testEmailConnection: { success: boolean; message: string } }>(TEST_EMAIL_CONNECTION_MUTATION);
      return data.testEmailConnection;
    },
    onSuccess: (result) => {
      if (result.success) {
        toast.success(i18n.t('system.email.testSuccess'));
      } else {
        toast.error(result.message || i18n.t('system.email.testFailed'));
      }
    },
    onError: () => {
      toast.error(i18n.t('system.email.testFailed'));
    },
  });
}
