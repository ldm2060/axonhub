import { createFileRoute, redirect } from '@tanstack/react-router';
import { getTokenFromStorage } from '@/stores/authStore';
import SignIn from '@/features/auth/sign-in';

export const Route = createFileRoute('/(auth)/sign-in')({
  validateSearch: (search: Record<string, unknown>): { redirect?: string } => ({
    redirect: typeof search.redirect === 'string' ? search.redirect : undefined,
  }),
  beforeLoad: () => {
    if (getTokenFromStorage()) {
      throw redirect({ to: '/' });
    }
  },
  component: SignIn,
});
