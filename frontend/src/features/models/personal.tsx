import { useState, useMemo, useCallback, useEffect } from 'react';
import { SortingState } from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { usePermissions } from '@/hooks/usePermissions';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { LazyModelsDialogs } from './components/lazy-models-dialogs';
import { createColumns } from './components/models-columns';
import { ModelsPersonalButtons } from './components/models-personal-buttons';
import { ModelsTable } from './components/models-table';
import ModelsProvider from './context/models-provider';
import { useQueryAllModels } from './data/models';
import { useDevelopersData } from './data/providers';

function PersonalModelsContent() {
  useDevelopersData();
  const { t } = useTranslation();
  const { modelPermissions } = usePermissions();

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
            <p className='text-muted-foreground text-sm'>{t('models.personal.description')}</p>
          </div>
          <ModelsPersonalButtons />
        </div>
      </Header>

      <Main fixed>
        <PersonalModelsContent />
      </Main>
      <LazyModelsDialogs />
    </ModelsProvider>
  );
}
