import { createFileRoute, Outlet, redirect } from '@tanstack/react-router';
import i18next from 'i18next';
import { toast } from 'sonner';
import { hasPathAccess } from '@/config/route-permission';
import { useAuthStore } from '@/stores/authStore';
import { useProjectStore } from '@/stores/projectStore';

export const Route = createFileRoute('/_authenticated/admin')({
  beforeLoad: ({ location }) => {
    const { user } = useAuthStore.getState().auth;
    const selectedProjectId = useProjectStore.getState().selectedProjectId;
    if (!hasPathAccess(user, location.pathname, selectedProjectId)) {
      toast.error(i18next.t('common.errors.noAdminPermission'));
      throw redirect({ to: '/' });
    }
  },
  component: () => <Outlet />,
});
