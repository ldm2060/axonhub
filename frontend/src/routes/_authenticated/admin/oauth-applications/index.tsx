import { createFileRoute } from '@tanstack/react-router';
import OAuthApplicationsManagement from '@/features/oauth-applications';

export const Route = createFileRoute('/_authenticated/admin/oauth-applications/')({
  component: OAuthApplicationsManagement,
});
