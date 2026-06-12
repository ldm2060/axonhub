import { createFileRoute } from '@tanstack/react-router';
import RequestsManagement from '@/features/requests';

export const Route = createFileRoute('/_authenticated/admin/requests/')({
  validateSearch: (search: Record<string, unknown>) => search,
  component: () => <RequestsManagement scope='admin' />,
});
