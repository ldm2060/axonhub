import { createFileRoute } from '@tanstack/react-router';
import Dashboard from '@/features/dashboard';

function PersonalDashboard() {
  return <Dashboard mode="personal" />;
}

export const Route = createFileRoute('/_authenticated/')({
  component: PersonalDashboard,
});
