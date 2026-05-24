import { useState, useEffect, useMemo, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import {
  ColumnDef,
  useReactTable,
  getCoreRowModel,
  SortingState,
  flexRender,
} from '@tanstack/react-table';
import { Search, ArrowUpDown } from 'lucide-react';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '@/components/ui/select';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { DataTableColumnHeader } from '@/components/data-table-column-header';
import { ServerSidePagination } from '@/components/server-side-pagination';
import { Skeleton } from '@/components/ui/skeleton';
import { formatNumber } from '@/utils/format-number';
import {
  useUserUsageStats,
  type UserUsageStat,
  type TimeRange,
  type UserStatsSortField,
} from '../data/user-usage-stats';
import type { PageInfo } from '@/gql/pagination';

const sortByToColumn: Record<UserStatsSortField, string> = {
  REQUEST_COUNT: 'requestCount',
  TOTAL_COST: 'totalCost',
  TOTAL_TOKENS: 'totalTokens',
  LAST_ACTIVE_AT: 'lastActiveAt',
};

const columnToSortBy: Record<string, UserStatsSortField> = {
  requestCount: 'REQUEST_COUNT',
  totalCost: 'TOTAL_COST',
  totalTokens: 'TOTAL_TOKENS',
  lastActiveAt: 'LAST_ACTIVE_AT',
};

export function UserUsageStatsSection() {
  const { t } = useTranslation();

  // State
  const [timeRange, setTimeRange] = useState<TimeRange>('ALL');
  const [searchInput, setSearchInput] = useState('');
  const [search, setSearch] = useState('');
  const [sortBy, setSortBy] = useState<UserStatsSortField>('REQUEST_COUNT');
  const [sortOrder, setSortOrder] = useState<'ASC' | 'DESC'>('DESC');
  const [page, setPage] = useState(1);
  const [pageSize, setPageSize] = useState(20);

  // Derived sorting state for the table
  const sorting: SortingState = useMemo(
    () => [{ id: sortByToColumn[sortBy], desc: sortOrder === 'DESC' }],
    [sortBy, sortOrder],
  );

  // Handle column header sort clicks
  const handleSortingChange = useCallback(
    (updaterOrValue: SortingState | ((old: SortingState) => SortingState)) => {
      const newSorting =
        typeof updaterOrValue === 'function' ? updaterOrValue(sorting) : updaterOrValue;
      if (newSorting.length > 0) {
        const newSortBy = columnToSortBy[newSorting[0].id];
        if (newSortBy) {
          setSortBy(newSortBy);
          setSortOrder(newSorting[0].desc ? 'DESC' : 'ASC');
          setPage(1);
        }
      }
    },
    [sorting],
  );

  // Debounced search
  useEffect(() => {
    const timer = setTimeout(() => {
      setSearch(searchInput);
      setPage(1);
    }, 300);
    return () => clearTimeout(timer);
  }, [searchInput]);

  // Data fetching
  const { data, isLoading } = useUserUsageStats(timeRange, search, sortBy, sortOrder, page, pageSize);

  // Compute page info from data
  const pageInfo: PageInfo = useMemo(
    () => ({
      hasNextPage: (data?.stats.length ?? 0) === pageSize,
      hasPreviousPage: page > 1,
      startCursor: null,
      endCursor: null,
    }),
    [data?.stats.length, page, pageSize],
  );

  // Columns
  const columns: ColumnDef<UserUsageStat>[] = useMemo(
    () => [
      {
        accessorKey: 'userName',
        enableSorting: false,
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('userStats.columns.userName')} />
        ),
        cell: ({ row }) => (
          <div className="font-medium">{row.getValue('userName')}</div>
        ),
      },
      {
        accessorKey: 'userEmail',
        enableSorting: false,
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('userStats.columns.email')} />
        ),
        cell: ({ row }) => (
          <div className="text-muted-foreground text-xs">{row.getValue('userEmail')}</div>
        ),
      },
      {
        accessorKey: 'requestCount',
        enableSorting: true,
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('userStats.columns.requests')} />
        ),
        cell: ({ row }) => (
          <div className="font-mono text-sm">{formatNumber(row.getValue('requestCount') as number)}</div>
        ),
      },
      {
        accessorKey: 'successRate',
        enableSorting: false,
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('userStats.columns.successRate')} />
        ),
        cell: ({ row }) => {
          const rate = row.getValue('successRate') as number;
          return <div className="font-mono text-sm">{rate.toFixed(1)}%</div>;
        },
      },
      {
        accessorKey: 'totalTokens',
        enableSorting: true,
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('userStats.columns.tokens')} />
        ),
        cell: ({ row }) => (
          <div className="font-mono text-sm">{formatNumber(row.getValue('totalTokens') as number)}</div>
        ),
      },
      {
        accessorKey: 'totalCost',
        enableSorting: true,
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('userStats.columns.cost')} />
        ),
        cell: ({ row }) => {
          const cost = row.getValue('totalCost') as number;
          return <div className="font-mono text-sm">${cost.toFixed(4)}</div>;
        },
      },
      {
        accessorKey: 'lastActiveAt',
        enableSorting: true,
        header: ({ column }) => (
          <DataTableColumnHeader column={column} title={t('userStats.columns.lastActive')} />
        ),
        cell: ({ row }) => {
          const lastActive = row.getValue('lastActiveAt') as string | null;
          if (!lastActive) {
            return <div className="text-muted-foreground text-xs">-</div>;
          }
          return (
            <div className="text-xs">
              {new Date(lastActive).toLocaleDateString()}
            </div>
          );
        },
      },
    ],
    [t],
  );

  const table = useReactTable({
    data: data?.stats ?? [],
    columns,
    state: { sorting },
    onSortingChange: handleSortingChange,
    manualSorting: true,
    getCoreRowModel: getCoreRowModel(),
  });

  // Pagination handlers
  const handleNextPage = useCallback(() => setPage((p) => p + 1), []);
  const handlePreviousPage = useCallback(() => setPage((p) => Math.max(1, p - 1)), []);
  const handleFirstPage = useCallback(() => setPage(1), []);
  const handlePageSizeChange = useCallback((newSize: number) => {
    setPageSize(newSize);
    setPage(1);
  }, []);

  const handleTimeRangeChange = useCallback((value: string) => {
    setTimeRange(value as TimeRange);
    setPage(1);
  }, []);

  const handleSortByChange = useCallback((value: string) => {
    setSortBy(value as UserStatsSortField);
    setPage(1);
  }, []);

  const handleSortOrderToggle = useCallback(() => {
    setSortOrder((prev) => (prev === 'ASC' ? 'DESC' : 'ASC'));
    setPage(1);
  }, []);

  const stats = data?.stats ?? [];

  return (
    <div className="space-y-4">
      {/* Summary Cards */}
      <div className="grid gap-4 md:grid-cols-3">
        <Card className="hover-card">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              {t('userStats.cards.totalUsers')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className="h-8 w-20" />
            ) : (
              <div className="text-2xl font-bold">
                {formatNumber(data?.totalUsers ?? 0)}
              </div>
            )}
          </CardContent>
        </Card>
        <Card className="hover-card">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              {t('userStats.cards.active7d')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className="h-8 w-20" />
            ) : (
              <div className="text-2xl font-bold">
                {formatNumber(data?.activeUsers7d ?? 0)}
              </div>
            )}
          </CardContent>
        </Card>
        <Card className="hover-card">
          <CardHeader className="flex flex-row items-center justify-between space-y-0 pb-2">
            <CardTitle className="text-sm font-medium">
              {t('userStats.cards.active30d')}
            </CardTitle>
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <Skeleton className="h-8 w-20" />
            ) : (
              <div className="text-2xl font-bold">
                {formatNumber(data?.activeUsers30d ?? 0)}
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Filter Toolbar */}
      <div className="flex flex-wrap items-center gap-3">
        <Select value={timeRange} onValueChange={handleTimeRangeChange}>
          <SelectTrigger className="h-9 w-[150px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="ALL">{t('userStats.timeRange.all')}</SelectItem>
            <SelectItem value="LAST_7D">{t('userStats.timeRange.last7d')}</SelectItem>
            <SelectItem value="LAST_30D">{t('userStats.timeRange.last30d')}</SelectItem>
          </SelectContent>
        </Select>

        <div className="relative flex-1 min-w-[200px] max-w-sm">
          <Search className="absolute left-2.5 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
          <Input
            placeholder={t('userStats.searchPlaceholder')}
            value={searchInput}
            onChange={(e) => setSearchInput(e.target.value)}
            className="h-9 pl-9"
          />
        </div>

        <Select value={sortBy} onValueChange={handleSortByChange}>
          <SelectTrigger className="h-9 w-[140px]">
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="REQUEST_COUNT">{t('userStats.sortBy.requestCount')}</SelectItem>
            <SelectItem value="TOTAL_COST">{t('userStats.sortBy.totalCost')}</SelectItem>
            <SelectItem value="TOTAL_TOKENS">{t('userStats.sortBy.totalTokens')}</SelectItem>
            <SelectItem value="LAST_ACTIVE_AT">{t('userStats.sortBy.lastActiveAt')}</SelectItem>
          </SelectContent>
        </Select>

        <Button
          variant="outline"
          size="sm"
          onClick={handleSortOrderToggle}
          className="h-9 gap-1.5"
        >
          <ArrowUpDown className="h-3.5 w-3.5" />
          {sortOrder === 'ASC' ? t('userStats.sortOrder.asc') : t('userStats.sortOrder.desc')}
        </Button>
      </div>

      {/* Data Table */}
      <div className="rounded-md border">
        <Table>
          <TableHeader>
            {table.getHeaderGroups().map((headerGroup) => (
              <TableRow key={headerGroup.id}>
                {headerGroup.headers.map((header) => (
                  <TableHead key={header.id}>
                    {header.isPlaceholder
                      ? null
                      : flexRender(header.column.columnDef.header, header.getContext())}
                  </TableHead>
                ))}
              </TableRow>
            ))}
          </TableHeader>
          <TableBody>
            {isLoading ? (
              Array.from({ length: 5 }).map((_, i) => (
                <TableRow key={i}>
                  {columns.map((_col, j) => (
                    <TableCell key={j}>
                      <Skeleton className="h-5 w-full" />
                    </TableCell>
                  ))}
                </TableRow>
              ))
            ) : stats.length === 0 ? (
              <TableRow>
                <TableCell colSpan={columns.length} className="h-24 text-center">
                  <div className="flex flex-col items-center justify-center gap-2 text-muted-foreground">
                    <p>{t('userStats.noData')}</p>
                  </div>
                </TableCell>
              </TableRow>
            ) : (
              table.getRowModel().rows.map((row) => (
                <TableRow key={row.id}>
                  {row.getVisibleCells().map((cell) => (
                    <TableCell key={cell.id}>
                      {flexRender(cell.column.columnDef.cell, cell.getContext())}
                    </TableCell>
                  ))}
                </TableRow>
              ))
            )}
          </TableBody>
        </Table>
      </div>

      {/* Pagination */}
      <ServerSidePagination
        pageInfo={pageInfo}
        pageSize={pageSize}
        dataLength={stats.length}
        totalCount={undefined}
        selectedRows={0}
        onNextPage={handleNextPage}
        onPreviousPage={handlePreviousPage}
        onFirstPage={handleFirstPage}
        onPageSizeChange={handlePageSizeChange}
      />
    </div>
  );
}
