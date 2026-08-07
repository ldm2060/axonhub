import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import SystemManagement from '@/features/system';
import { isSystemTabKey } from '@/features/system/data/system-tabs';

function ProtectedSystem() {
  const search = Route.useSearch();

  return (
    <RouteGuard requiredScopes={['read_settings']} scopeLevel='system'>
      <SystemManagement initialTab={search.tab} />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/admin/system/')({
  component: ProtectedSystem,
  validateSearch: (search: Record<string, unknown>) => ({
    tab: isSystemTabKey(search.tab) ? search.tab : undefined,
  }),
});
