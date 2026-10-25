import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import RequestsManagement from '@/features/requests';

export const Route = createFileRoute('/_authenticated/admin/requests/')({
  validateSearch: (search: Record<string, unknown>) => search,
  component: () => (
    <RouteGuard requiredScopes={['read_requests']} scopeLevel='system'>
      <RequestsManagement scope='admin' />
    </RouteGuard>
  ),
});
