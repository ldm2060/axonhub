import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import SystemManagement from '@/features/system';

type SystemTabKey =
  | 'general'
  | 'security'
  | 'brand'
  | 'registration'
  | 'email'
  | 'storage'
  | 'retry'
  | 'streaming'
  | 'webhook'
  | 'proxy'
  | 'quota'
  | 'backup'
  | 'diagnostics'
  | 'about';

function ProtectedSystem() {
  const search = Route.useSearch();

  return (
    <RouteGuard requiredScopes={['read_settings']} scopeLevel='system'>
      <SystemManagement initialTab={search.tab as SystemTabKey | undefined} />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/admin/system/')({
  component: ProtectedSystem,
  validateSearch: (search: { tab?: SystemTabKey }) => search,
});
