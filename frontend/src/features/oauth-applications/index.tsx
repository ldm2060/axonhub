import { useMemo, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { useDebounce } from '@/hooks/use-debounce';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { createColumns } from './components/oauth-applications-columns';
import { OAuthApplicationDialogs } from './components/oauth-application-dialogs';
import { OAuthApplicationsPrimaryButtons } from './components/oauth-applications-primary-buttons';
import { OAuthApplicationsTable } from './components/oauth-applications-table';
import OAuthApplicationsProvider from './context/oauth-applications-provider';
import { useOAuthApplications } from './data/oauth-applications';

function OAuthApplicationsContent() {
  const { t } = useTranslation();
  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'oauth-applications-table-page-size',
  });
  const [nameFilter, setNameFilter] = useState<string>('');

  const debouncedNameFilter = useDebounce(nameFilter, 300);

  const whereClause = (() => {
    const where: Record<string, string> = {};
    if (debouncedNameFilter) {
      where.nameContainsFold = debouncedNameFilter;
    }
    return Object.keys(where).length > 0 ? where : undefined;
  })();

  const { data } = useOAuthApplications({
    ...paginationArgs,
    where: whereClause,
    orderBy: {
      field: 'CREATED_AT',
      direction: 'DESC',
    },
  });

  const handleNextPage = () => {
    if (data?.pageInfo?.hasNextPage && data?.pageInfo?.endCursor) {
      setCursors(data.pageInfo.startCursor ?? undefined, data.pageInfo.endCursor ?? undefined, 'after');
    }
  };

  const handlePreviousPage = () => {
    if (data?.pageInfo?.hasPreviousPage) {
      setCursors(data.pageInfo.startCursor ?? undefined, data.pageInfo.endCursor ?? undefined, 'before');
    }
  };

  const handlePageSizeChange = (newPageSize: number) => {
    setPageSize(newPageSize);
  };

  const handleNameFilterChange = (filter: string) => {
    setNameFilter(filter);
    resetCursor();
  };

  // Memoize columns to prevent infinite re-renders (React #185)
  const columns = useMemo(() => createColumns(t), [t]);

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <OAuthApplicationsTable
        data={data?.edges?.map((edge) => edge.node) || []}
        columns={columns}
        pageInfo={data?.pageInfo}
        pageSize={pageSize}
        totalCount={data?.totalCount}
        nameFilter={nameFilter}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        onNameFilterChange={handleNameFilterChange}
      />
    </div>
  );
}

export default function OAuthApplicationsManagement() {
  const { t } = useTranslation();

  return (
    <OAuthApplicationsProvider>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between'>
          <div>
            <h2 className='text-xl font-bold tracking-tight'>{t('oauthApplications.title')}</h2>
            <p className='text-muted-foreground text-sm'>{t('oauthApplications.description')}</p>
          </div>
          <OAuthApplicationsPrimaryButtons />
        </div>
      </Header>

      <Main fixed>
        <OAuthApplicationsContent />
      </Main>
      <OAuthApplicationDialogs />
    </OAuthApplicationsProvider>
  );
}
