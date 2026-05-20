import { createFileRoute } from '@tanstack/react-router';
import { RouteGuard } from '@/components/route-guard';
import PublishRequests from '@/features/publish-requests';

function ProtectedPublishRequests() {
  return (
    <RouteGuard requiredScopes={['read_channels']}>
      <PublishRequests />
    </RouteGuard>
  );
}

export const Route = createFileRoute('/_authenticated/admin/publish-requests/')({
  component: ProtectedPublishRequests,
});
