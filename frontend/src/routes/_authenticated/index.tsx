import { createFileRoute } from '@tanstack/react-router';
import { DashboardModeContext } from '@/features/dashboard/context';
import type { DashboardMode } from '@/features/dashboard/data/dashboard';
import Dashboard from '@/features/dashboard';

function PersonalDashboard() {
  return (
    <DashboardModeContext.Provider value={'personal' as DashboardMode}>
      <Dashboard />
    </DashboardModeContext.Provider>
  );
}

export const Route = createFileRoute('/_authenticated/')({
  component: PersonalDashboard,
});
