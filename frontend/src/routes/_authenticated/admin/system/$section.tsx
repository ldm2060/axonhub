import { createFileRoute, redirect } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import SystemManagement from '@/features/system';
import { isSystemSubPageKey, type SystemTabKey } from '@/features/system/data/system-tabs';

function ProtectedSystemPage() {
  const { section } = Route.useParams();
  const search = Route.useSearch();
  if (!isSystemSubPageKey(section)) {
    return null;
  }

  return (
    <RouteGuard requiredScopes={['read_settings']} scopeLevel='system'>
      <SystemManagement page={section} initialTab={search.tab} />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/admin/system/$section')({
  beforeLoad: ({ params }) => {
    if (!isSystemSubPageKey(params.section)) {
      throw redirect({ to: '/admin/system', search: { tab: undefined } });
    }
  },
  validateSearch: (search: { tab?: SystemTabKey }) => search,
  component: ProtectedSystemPage,
});
