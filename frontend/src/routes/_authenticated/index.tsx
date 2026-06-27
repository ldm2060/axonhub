import { createFileRoute } from '@tanstack/react-router';
import Dashboard from '@/features/dashboard';
import { RouteGuard } from '@/components/route-guard';

function PersonalDashboard() {
  return (
    <RouteGuard requiredScopes={['read_dashboard']} scopeLevel="system">
      <Dashboard mode="personal" />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/')({
  component: PersonalDashboard,
});
