import { createFileRoute } from '@tanstack/react-router';
import UsageMonitorPage from '@/features/usage-monitor';

export const Route = createFileRoute('/_authenticated/admin/usage-monitor/')({
  component: UsageMonitorPage,
});
