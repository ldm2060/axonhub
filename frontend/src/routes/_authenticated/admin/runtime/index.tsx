import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import RuntimeManagement from '@/features/runtime';

export const Route = createFileRoute('/_authenticated/admin/runtime/')({
  component: () => (
    <RouteGuard requiredScopes={['read_settings']} scopeLevel='system'>
      <RuntimeManagement />
    </RouteGuard>
  ),
});
