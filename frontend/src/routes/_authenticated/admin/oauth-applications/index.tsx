import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import OAuthApplicationsManagement from '@/features/oauth-applications';

export const Route = createFileRoute('/_authenticated/admin/oauth-applications/')({
  component: () => (
    <RouteGuard requiredScopes={['read_settings']} scopeLevel='system'>
      <OAuthApplicationsManagement />
    </RouteGuard>
  ),
});
