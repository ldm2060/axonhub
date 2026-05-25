import { useEffect } from 'react';
import { useMutation, useQuery } from '@tanstack/react-query';
import { useRouter } from '@tanstack/react-router';
import { graphqlRequest } from '@/gql/graphql';
import { ME_QUERY } from '@/gql/users';
import { toast } from 'sonner';
import { useAuthStore, setTokenToStorage, removeTokenFromStorage } from '@/stores/authStore';
import { AuthUser } from '@/stores/authStore';
import { authApi } from '@/lib/api-client';
import i18n from '@/lib/i18n';

export interface SignInInput {
  email: string;
  password: string;
}

interface MeResponse {
  me: AuthUser;
}

export function useMe(enabled = true) {
  const { setUser } = useAuthStore((state) => state.auth);

  const query = useQuery({
    queryKey: ['me'],
    queryFn: async () => {
      const data = await graphqlRequest<MeResponse>(ME_QUERY);
      return data.me;
    },
    enabled: enabled && !!useAuthStore.getState().auth.accessToken,
    retry: false,
  });

  // Update auth store when data changes
  useEffect(() => {
    if (query.data) {
      const userLanguage = query.data.preferLanguage || 'en';

      setUser(query.data);

      // Initialize i18n with user's preferred language
      if (userLanguage !== i18n.language) {
        i18n.changeLanguage(userLanguage);
      }
    }
  }, [query.data, setUser]);

  return query;
}

export function useSignIn() {
  const { setUser, setAccessToken } = useAuthStore((state) => state.auth);
  const router = useRouter();

  return useMutation({
    mutationFn: async (input: SignInInput) => {
      return await authApi.signIn(input);
    },
    onSuccess: (data) => {
      // Store token in localStorage
      setTokenToStorage(data.token);

      const userLanguage = data.user.preferLanguage || 'en';

      // Update auth store
      setAccessToken(data.token);
      setUser(data.user);

      // Initialize i18n with user's preferred language
      if (userLanguage !== i18n.language) {
        i18n.changeLanguage(userLanguage);
      }

      toast.success(i18n.t('common.success.signedIn'));

      // Redirect based on user role
      // Owner users go to dashboard, non-owner users go to requests page
      const redirectPath = data.user.isOwner ? '/' : '/project/playground';
      router.navigate({ to: redirectPath });
    },
    onError: (error: any) => {
      const errorMessage = error.message || 'Failed to sign in';
      toast.error(errorMessage);
    },
  });
}

export function useSignOut() {
  const { reset } = useAuthStore((state) => state.auth);
  const router = useRouter();

  return () => {
    // Clear token from localStorage
    removeTokenFromStorage();

    // Clear auth store
    reset();

    toast.success(i18n.t('common.success.signedOut'));

    // Redirect to sign in page
    router.navigate({ to: '/sign-in' });
  };
}


export interface SignUpInput {
  email: string;
  password: string;
  first_name: string;
  last_name: string;
  verification_code: string;
}

export function useSignUpAllowed() {
  return useQuery({
    queryKey: ['sign-up-allowed'],
    queryFn: async () => {
      const result = await authApi.isSignUpAllowed();
      return result.allowed;
    },
    staleTime: 30 * 1000,
    retry: 1,
  });
}

export function useSendVerificationCode() {
  return useMutation({
    mutationFn: async (email: string) => {
      return await authApi.sendVerificationCode(email);
    },
    onError: (error: any) => {
      toast.error(error.message || 'Failed to send verification code');
    },
  });
}

export function useSignUp() {
  return useMutation({
    mutationFn: async (input: SignUpInput) => {
      return await authApi.signUp(input);
    },
    onError: (error: any) => {
      const errorMessage = error.message || 'Failed to sign up';
      toast.error(errorMessage);
    },
  });
}

export function useOIDCProviders() {
  return useQuery({
    queryKey: ['oidc-providers'],
    queryFn: async () => {
      const response = await authApi.getOIDCProviders();
      return response.data || [];
    },
    staleTime: 5 * 60 * 1000, // 5 minutes
    retry: 1,
  });
}

export function useOIDCAuthorize() {
  return useMutation({
    mutationFn: async (providerId: string) => {
      return await authApi.getOIDCAuthorizeURL(providerId);
    },
    onSuccess: (response) => {
      if (response && response.data && response.data.url) {
        window.location.href = response.data.url;
      } else {
        toast.error('Invalid authorization URL received');
      }
    },
    onError: (error: unknown) => {
      const errorMessage = error instanceof Error ? error.message : 'Failed to initialize SSO login';
      toast.error(errorMessage);
    },
  });
}

export function useOIDCExchange() {
  const { setUser, setAccessToken } = useAuthStore((state) => state.auth);
  const router = useRouter();

  return useMutation({
    mutationFn: async (code: string) => {
      return await authApi.exchangeOIDCCode(code);
    },
    onSuccess: (response) => {
      const data = response.data;
      
      // Store token in localStorage
      setTokenToStorage(data.token);

      const userLanguage = data.user.preferLanguage || 'en';

      // Update auth store
      setAccessToken(data.token);
      setUser(data.user);

      // Initialize i18n with user's preferred language
      if (userLanguage !== i18n.language) {
        i18n.changeLanguage(userLanguage);
      }

      toast.success(i18n.t('common.success.signedIn'));

      // Redirect based on user role
      const redirectPath = data.user.isOwner ? '/' : '/project/playground';
      router.navigate({ to: redirectPath });
    },
    onError: (error: unknown) => {
      const errorMessage = error instanceof Error ? error.message : 'SSO login failed';
      toast.error(errorMessage);
      router.navigate({ to: '/sign-in' });
    },
  });
}
