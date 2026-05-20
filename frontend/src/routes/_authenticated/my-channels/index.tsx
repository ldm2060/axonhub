import { createFileRoute } from '@tanstack/react-router';
import PersonalChannelsPage from '@/features/channels/personal';

export const Route = createFileRoute('/_authenticated/my-channels/')({
  component: PersonalChannelsPage,
});
