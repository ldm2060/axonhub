import { createFileRoute } from '@tanstack/react-router';
import Invite from '@/features/auth/invite';

export const Route = createFileRoute('/(auth)/invite')({
  component: Invite,
  validateSearch: (search: Record<string, unknown>): { token?: string } => ({
    token: typeof search.token === 'string' ? search.token : undefined,
  }),
});
