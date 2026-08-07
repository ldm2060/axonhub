import { useState, useMemo, useCallback, useEffect, lazy, Suspense } from 'react';
import { SortingState } from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '@/stores/authStore';
import { useDebounce } from '@/hooks/use-debounce';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { usePermissions } from '@/hooks/usePermissions';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { useMe } from '@/features/auth/data/auth';
import { useProvidersData } from '@/features/models/data/providers';
import { createColumns } from './components/channels-columns';
import { ChannelsErrorBanner } from './components/channels-error-banner';
import { PersonalChannelsButtons } from './components/channels-personal-buttons';
import { ChannelsTable } from './components/channels-table';
import { ChannelsTypeTabs } from './components/channels-type-tabs';
import ChannelsProvider from './context/channels-context';
import { useQueryChannels, useChannelTypes, useErrorChannelsCount, useChannelProbeData } from './data/channels';
import { useMySharedChannels } from './data/shared';
import {
  type PersonalChannelSource,
  isPersonalChannelSourceReadOnly,
  buildPersonalChannelWhere,
  filterSharedPersonalChannels,
  filterOwnedPersonalChannels,
  filterPersonalChannelRows,
} from './personal-channel-source';

const ChannelsDialogs = lazy(() => import('./components/channels-dialogs').then((m) => ({ default: m.ChannelsDialogs })));

function PersonalChannelsContent() {
  const { t } = useTranslation();
  const { user: authUser } = useAuthStore((state) => state.auth);
  const { data: meData } = useMe();
  const currentUser = meData || authUser;

  useProvidersData();
  const { channelPermissions } = usePermissions();
  const canWrite = channelPermissions.canWrite;

  const [source, setSource] = useState<PersonalChannelSource>('mine');
  const isReadOnly = isPersonalChannelSourceReadOnly(source);

  const { pageSize, setCursors, setPageSize, resetCursor, paginationArgs } = usePaginationSearch({
    defaultPageSize: 20,
    pageSizeStorageKey: 'my-channels-table-page-size',
  });
  const [nameFilter, setNameFilter] = useState<string>('');
  const [typeFilter, setTypeFilter] = useState<string[]>([]);
  const [statusFilter, setStatusFilter] = useState<string[]>([]);
  const [tagFilter, setTagFilter] = useState<string>('');
  const [modelFilter, setModelFilter] = useState<string>('');
  const [selectedTypeTab, setSelectedTypeTab] = useState<string>('all');
  const [showErrorOnly, setShowErrorOnly] = useState<boolean>(false);
  const [sorting, setSorting] = useState<SortingState>(() => {
    const stored = localStorage.getItem('my-channels-table-sorting');
    if (stored) {
      try {
        return JSON.parse(stored);
      } catch {
        return [{ id: 'createdAt', desc: true }];
      }
    }
    return [{ id: 'createdAt', desc: true }];
  });
  const [isHealthColumnVisible, setIsHealthColumnVisible] = useState<boolean>(() => {
    const stored = localStorage.getItem('my-channels-table-column-visibility');
    if (stored) {
      try {
        const visibility = JSON.parse(stored);
        return visibility.health !== false;
      } catch {
        return true;
      }
    }
    return true;
  });

  useEffect(() => {
    localStorage.setItem('my-channels-table-sorting', JSON.stringify(sorting));
  }, [sorting]);

  const { data: channelTypeCounts = [] } = useChannelTypes(
    statusFilter.length > 0 ? statusFilter : ['enabled', 'disabled'],
    currentUser?.id
  );
  const { data: errorCount = 0 } = useErrorChannelsCount(currentUser?.id);
  const debouncedNameFilter = useDebounce(nameFilter, 300);

  const tabFilteredTypes = useMemo(() => {
    if (selectedTypeTab === 'all') {
      return [];
    }
    return channelTypeCounts.filter(({ type }) => type.startsWith(selectedTypeTab)).map(({ type }) => type);
  }, [selectedTypeTab, channelTypeCounts]);

  const whereClause = useMemo(() => {
    const base: Record<string, string | string[] | boolean> = {};
    if (debouncedNameFilter) {
      base.nameContainsFold = debouncedNameFilter;
    }
    const combinedTypeFilter = [...typeFilter];
    if (tabFilteredTypes.length > 0) {
      combinedTypeFilter.push(...tabFilteredTypes);
    }
    if (combinedTypeFilter.length > 0) {
      base.typeIn = Array.from(new Set(combinedTypeFilter));
    }
    if (statusFilter.length > 0) {
      base.statusIn = statusFilter;
    }
    if (showErrorOnly) {
      base.errorMessageNotNil = true;
    }
    return base;
  }, [debouncedNameFilter, tabFilteredTypes, typeFilter, statusFilter, showErrorOnly]);

  const mineWhereClause = useMemo(() => {
    return buildPersonalChannelWhere('mine', String(currentUser?.id), whereClause);
  }, [currentUser?.id, whereClause]);

  const publicWhereClause = useMemo(() => {
    return buildPersonalChannelWhere('public', String(currentUser?.id), whereClause);
  }, [currentUser?.id, whereClause]);

  const currentOrderBy = useMemo(() => {
    if (sorting.length === 0) {
      return { field: 'CREATED_AT', direction: 'DESC' } as const;
    }
    const [primary] = sorting;
    switch (primary.id) {
      case 'name':
        return { field: 'NAME', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      case 'status':
        return { field: 'STATUS', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      case 'provider':
      case 'type':
        return { field: 'TYPE', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      case 'createdAt':
        return { field: 'CREATED_AT', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      case 'updatedAt':
        return { field: 'UPDATED_AT', direction: primary.desc ? 'DESC' : 'ASC' } as const;
      default:
        return { field: 'CREATED_AT', direction: 'DESC' } as const;
    }
  }, [sorting]);

  const { data: mineData, isLoading: mineLoading } = useQueryChannels({
    ...paginationArgs,
    where: mineWhereClause,
    orderBy: currentOrderBy,
    hasTag: tagFilter || undefined,
    model: modelFilter || undefined,
  });

  const { data: publicData, isLoading: publicLoading } = useQueryChannels({
    ...paginationArgs,
    where: publicWhereClause,
    orderBy: currentOrderBy,
    hasTag: tagFilter || undefined,
    model: modelFilter || undefined,
  });

  const { data: sharedRaw = [], isLoading: sharedLoading } = useMySharedChannels();
  const sharedChannels = useMemo(() => filterSharedPersonalChannels(sharedRaw, currentUser?.id), [sharedRaw, currentUser?.id]);
  const sharedFiltered = useMemo(
    () =>
      filterPersonalChannelRows(sharedChannels, {
        nameContainsFold: debouncedNameFilter,
        typeIn: typeFilter.length > 0 ? typeFilter : tabFilteredTypes.length > 0 ? tabFilteredTypes : [],
        hasTag: tagFilter || undefined,
        model: modelFilter || undefined,
      }),
    [sharedChannels, debouncedNameFilter, typeFilter, tabFilteredTypes, tagFilter, modelFilter]
  );

  const publicDataFiltered = useMemo(() => {
    if (!publicData?.edges) return publicData;
    const filteredEdges = publicData.edges.filter((edge) => filterOwnedPersonalChannels([edge.node], currentUser?.id).length > 0);
    return {
      ...publicData,
      edges: filteredEdges,
      totalCount: publicData.totalCount - (publicData.edges.length - filteredEdges.length),
    };
  }, [publicData, currentUser?.id]);

  const activeData = useMemo(() => {
    if (source === 'mine') return mineData;
    if (source === 'public') return publicDataFiltered;
    return undefined;
  }, [source, mineData, publicDataFiltered]);

  const isLoading = source === 'shared' ? sharedLoading : source === 'public' ? publicLoading : mineLoading;
  const activePageInfo = source === 'shared' ? undefined : activeData?.pageInfo;
  const activeTotalCount = source === 'shared' ? sharedFiltered.length : activeData?.totalCount;

  const channelIDs = useMemo(() => {
    if (source === 'shared') return sharedFiltered.map((c) => c.id);
    return activeData?.edges?.map((edge) => edge.node.id) || [];
  }, [source, activeData?.edges, sharedFiltered]);

  const { data: probeData } = useChannelProbeData(channelIDs, { enabled: isHealthColumnVisible });

  const channelsWithProbeData = useMemo(() => {
    if (source === 'shared') {
      const probeMap = new Map(probeData?.map((probe) => [probe.channelID, probe.points]) || []);
      return sharedFiltered.map((channel) => ({
        ...channel,
        probePoints: probeMap.get(channel.id) || [],
      }));
    }
    if (!activeData?.edges) return [];
    const probeMap = new Map(probeData?.map((probe) => [probe.channelID, probe.points]) || []);
    return activeData.edges.map((edge) => ({
      ...edge.node,
      probePoints: probeMap.get(edge.node.id) || [],
    }));
  }, [source, activeData?.edges, sharedFiltered, probeData]);

  const handleNextPage = useCallback(() => {
    if (activeData?.pageInfo?.hasNextPage && activeData?.pageInfo?.endCursor) {
      setCursors(activeData.pageInfo.startCursor ?? undefined, activeData.pageInfo.endCursor ?? undefined, 'after');
    }
  }, [activeData?.pageInfo, setCursors]);

  const handlePreviousPage = useCallback(() => {
    if (activeData?.pageInfo?.hasPreviousPage) {
      setCursors(activeData.pageInfo.startCursor ?? undefined, activeData.pageInfo.endCursor ?? undefined, 'before');
    }
  }, [activeData?.pageInfo, setCursors]);

  const handlePageSizeChange = useCallback(
    (newPageSize: number) => {
      setPageSize(newPageSize);
    },
    [setPageSize]
  );

  const handleNameFilterChange = useCallback(
    (filter: string) => {
      setNameFilter(filter);
      resetCursor();
    },
    [resetCursor]
  );

  const handleTypeFilterChange = useCallback(
    (filters: string[]) => {
      setTypeFilter(filters);
      resetCursor();
    },
    [resetCursor]
  );

  const handleTabChange = useCallback(
    (tab: string) => {
      setSelectedTypeTab(tab);
      setTypeFilter([]);
      resetCursor();
    },
    [resetCursor]
  );

  const handleStatusFilterChange = useCallback(
    (filters: string[]) => {
      setStatusFilter(filters);
      resetCursor();
    },
    [resetCursor]
  );

  const handleTagFilterChange = useCallback(
    (filter: string) => {
      setTagFilter(filter);
      resetCursor();
    },
    [resetCursor]
  );

  const handleModelFilterChange = useCallback(
    (filter: string) => {
      setModelFilter(filter);
      resetCursor();
    },
    [resetCursor]
  );

  const handleFilterErrorChannels = useCallback(() => {
    setShowErrorOnly(true);
    resetCursor();
  }, [resetCursor]);

  const handleExitErrorOnlyMode = useCallback(() => {
    setShowErrorOnly(false);
    resetCursor();
  }, [resetCursor]);

  const handleSourceChange = useCallback(
    (newSource: PersonalChannelSource) => {
      setSource(newSource);
      setTypeFilter([]);
      setSelectedTypeTab('all');
      resetCursor();
    },
    [resetCursor]
  );

  const columns = useMemo(() => createColumns(t, isReadOnly ? false : canWrite, { hideOrderingWeight: true }), [t, isReadOnly, canWrite]);

  return (
    <div className='flex flex-1 flex-col overflow-hidden'>
      <ChannelsErrorBanner
        errorCount={errorCount}
        onFilterErrorChannels={handleFilterErrorChannels}
        showErrorOnly={showErrorOnly}
        onExitErrorOnlyMode={handleExitErrorOnlyMode}
      />
      <div className='mb-6 w-full overflow-hidden'>
        <div className='hide-scroll flex flex-nowrap items-center gap-2 overflow-x-auto scroll-smooth'>
          {(['public', 'shared', 'mine'] as const).map((s) => (
            <button
              key={s}
              onClick={() => handleSourceChange(s)}
              className={`flex shrink-0 items-center gap-2 rounded-full px-4 py-1.5 text-sm font-medium whitespace-nowrap transition-all ${
                source === s
                  ? 'bg-primary text-primary-foreground shadow-primary/20 shadow-md'
                  : 'bg-card border-border text-foreground hover:border-primary hover:text-primary border'
              }`}
            >
              {t(`channels.sources.${s}`)}
            </button>
          ))}
        </div>
      </div>
      {source === 'mine' && <ChannelsTypeTabs typeCounts={channelTypeCounts} selectedTab={selectedTypeTab} onTabChange={handleTabChange} />}
      <ChannelsTable
        loading={isLoading}
        data={channelsWithProbeData}
        columns={columns}
        pageInfo={activePageInfo}
        pageSize={pageSize}
        totalCount={activeTotalCount}
        nameFilter={nameFilter}
        typeFilter={typeFilter}
        statusFilter={statusFilter}
        tagFilter={tagFilter}
        modelFilter={modelFilter}
        selectedTypeTab={selectedTypeTab}
        showErrorOnly={showErrorOnly}
        sorting={sorting}
        onSortingChange={setSorting}
        onExitErrorOnlyMode={handleExitErrorOnlyMode}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onPageSizeChange={handlePageSizeChange}
        onResetCursor={resetCursor}
        onNameFilterChange={handleNameFilterChange}
        onTypeFilterChange={handleTypeFilterChange}
        onStatusFilterChange={handleStatusFilterChange}
        onTagFilterChange={handleTagFilterChange}
        onModelFilterChange={handleModelFilterChange}
        onHealthColumnVisibilityChange={setIsHealthColumnVisible}
        canWrite={isReadOnly ? false : canWrite}
        showStatusFilter={!isReadOnly}
      />
    </div>
  );
}

export default function PersonalChannelsPage() {
  const { t } = useTranslation();

  return (
    <ChannelsProvider>
      <Header fixed>
        <div className='flex w-full flex-1 flex-col gap-2 md:flex-row md:items-center md:justify-between md:gap-0'>
          <div className='min-w-0'>
            <h2 className='text-xl font-bold tracking-tight'>{t('channels.personal.title')}</h2>
            <p className='text-muted-foreground text-sm'>{t('channels.personal.description')}</p>
          </div>
          <PersonalChannelsButtons />
        </div>
      </Header>

      <Main fixed>
        <PersonalChannelsContent />
      </Main>
      <Suspense fallback={null}>
        <ChannelsDialogs />
      </Suspense>
    </ChannelsProvider>
  );
}
