import { createFileRoute, Outlet, redirect } from '@tanstack/react-router';
import { useAuthStore } from '@/stores/authStore';
import { toast } from 'sonner';
import i18next from 'i18next';

export const Route = createFileRoute('/_authenticated/admin')({
  beforeLoad: () => {
    const { user } = useAuthStore.getState().auth;
    if (!user?.isOwner) {
      toast.error(i18next.t('common.errors.noAdminPermission'));
      throw redirect({ to: '/' });
    }
  },
  component: () => <Outlet />,
});
