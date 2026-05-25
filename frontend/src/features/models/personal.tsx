import { useState, useMemo, useCallback, useEffect, lazy, Suspense } from 'react';
import { SortingState } from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { usePermissions } from '@/hooks/usePermissions';
import { useAuthStore } from '@/stores/authStore';
import { useMe } from '@/features/auth/data/auth';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { createColumns } from './components/models-columns';
import { ModelsPersonalButtons } from './components/models-personal-buttons';
import { ModelsTable } from './components/models-table';
import ModelsProvider from './context/models-context';
import { useQueryAllModels } from './data/models';
import { useDevelopersData } from './data/providers';

const ModelsDialogs = lazy(() => import('./components/models-dialogs').then((m) => ({ default: m.ModelsDialogs })));

function PersonalModelsContent() {
  useDevelopersData();
  const { t } = useTranslation();
  const { modelPermissions } = usePermissions();
  const { user: authUser } = useAuthStore((state) => state.auth);
  const { data: meData } = useMe();
  const currentUser = meData || authUser;

  const [nameFilter, setNameFilter] = useState<string>('');
  const [sorting, setSorting] = useState<SortingState>(() => {
    const stored = localStorage.getItem('my-models-table-sorting');
    if (stored) {
      try {
        return JSON.parse(stored);
      } catch {
        return [{ id: 'name', desc: false }];
      }
    }
    return [{ id: 'name', desc: false }];
  });

  useEffect(() => {
    localStorage.setItem('my-models-table-sorting', JSON.stringify(sorting));
  }, [sorting]);

  const debouncedNameFilter = useDebounce(nameFilter, 300);

  const whereClause = useMemo(() => {
    const where: Record<string, unknown> = {};
    if (debouncedNameFilter) {
      where.or = [{ nameContainsFold: debouncedNameFilter }, { modelIDContainsFold: debouncedNameFilter }];
    }
    return Object.keys(where).length > 0 ? where : undefined;
  }, [debouncedNameFilter]);

  const { data, isLoading } = useQueryAllModels({
    where: whereClause,
  });

  const handleNameFilterChange = useCallback((filter: string) => {
    setNameFilter(filter);
  }, []);

  const columns = useMemo(() => createColumns(t, modelPermissions.canWrite), [t, modelPermissions.canWrite]);

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <ModelsTable
        data={data?.edges?.map((edge) => edge.node) || []}
        columns={columns}
        loading={isLoading}
        totalCount={data?.totalCount}
        nameFilter={nameFilter}
        sorting={sorting}
        onSortingChange={setSorting}
        onNameFilterChange={handleNameFilterChange}
        canWrite={modelPermissions.canWrite}
      />
    </div>
  );
}

export default function PersonalModelsPage() {
  const { t } = useTranslation();

  return (
    <ModelsProvider>
      <Header fixed>
        <div className='flex w-full flex-1 flex-col gap-2 md:flex-row md:items-center md:justify-between md:gap-0'>
          <div className='min-w-0'>
            <h2 className='text-xl font-bold tracking-tight'>{t('models.personal.title')}</h2>
            <p className='text-sm text-muted-foreground'>{t('models.personal.description')}</p>
          </div>
          <ModelsPersonalButtons />
        </div>
      </Header>

      <Main fixed>
        <PersonalModelsContent />
      </Main>
      <Suspense fallback={null}>
        <ModelsDialogs />
      </Suspense>
    </ModelsProvider>
  );
}
