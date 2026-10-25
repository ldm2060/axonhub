import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import RequestDetailGlobalPage from '@/features/requests/components/request-detail-global-page';

export const Route = createFileRoute('/_authenticated/admin/requests/$requestId')({
  validateSearch: (search: Record<string, unknown>) => search,
  component: () => (
    <RouteGuard requiredScopes={['read_requests']} scopeLevel='system'>
      <RequestDetailGlobalPage backTo='/admin/requests' />
    </RouteGuard>
  ),
});
