import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import UsageMonitorPage from '@/features/usage-monitor';

export const Route = createFileRoute('/_authenticated/admin/usage-monitor/')({
  component: () => (
    <RouteGuard requiredScopes={['read_channels']} scopeLevel='system'>
      <UsageMonitorPage />
    </RouteGuard>
  ),
});
