import { createFileRoute } from '@tanstack/react-router';
import PersonalModelsPage from '@/features/models/personal';

export const Route = createFileRoute('/_authenticated/my-models/')({
  component: PersonalModelsPage,
});
