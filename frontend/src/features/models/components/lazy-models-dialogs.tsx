import { lazy, Suspense } from 'react';
import { useModels } from '../context/models-context';

const ModelsDialogs = lazy(() => import('./models-dialogs').then((module) => ({ default: module.ModelsDialogs })));

export function LazyModelsDialogs() {
  const { open } = useModels();

  if (open === null) {
    return null;
  }

  return (
    <Suspense fallback={null}>
      <ModelsDialogs />
    </Suspense>
  );
}
