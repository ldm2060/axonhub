import { useEffect } from 'react';
import { createFileRoute, useRouter } from '@tanstack/react-router';
import { usePermissions } from '@/hooks/usePermissions';
import { RouteGuard } from '@/components/route-guard';
import RuntimeManagement from '@/features/runtime';

function ProtectedRuntime() {
  const router = useRouter();
  const { isOwner } = usePermissions();

  useEffect(() => {
    if (!isOwner) {
      router.navigate({ to: '/' });
    }
  }, [isOwner, router]);

  if (!isOwner) {
    return null;
  }

  return (
    <RouteGuard requiredScopes={['read_settings']} scopeLevel='system' showForbidden={false} fallbackPath='/'>
      <RuntimeManagement />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/admin/runtime/')({
  component: ProtectedRuntime,
});
