import { createFileRoute } from '@tanstack/react-router';
import SharedWithMePage from '@/features/shared';

export const Route = createFileRoute('/_authenticated/shared/')({
  component: SharedWithMePage,
});
