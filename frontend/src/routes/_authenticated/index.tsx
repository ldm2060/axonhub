import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import Dashboard from '@/features/dashboard';

function PersonalDashboard() {
  return (
    <RouteGuard>
      <Dashboard mode='personal' />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/')({
  component: PersonalDashboard,
});
