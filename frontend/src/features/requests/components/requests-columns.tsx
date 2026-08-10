'use client';

import { format } from 'date-fns';
import { ColumnDef } from '@tanstack/react-table';
import { IconArrowsJoin2, IconRoute } from '@tabler/icons-react';
import { zhCN, enUS } from 'date-fns/locale';
import { ArrowDown, ArrowUp, Ban, FileText } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { formatDuration } from '@/utils/format-duration';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { usePermissions } from '@/hooks/usePermissions';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { DataTableColumnHeader } from '@/components/data-table-column-header';
import { useGeneralSettings, useSecuritySettings, useUpdateSecuritySettings } from '@/features/system/data/system';
import { useRequestPermissions } from '../../../hooks/useRequestPermissions';
import { Request } from '../data/schema';
import { calculateTokensPerSecond, getTokensPerSecondValue } from '../utils/tokens-per-second';
import { getStatusColor } from './help';

interface UseRequestsColumnsOptions {
  onBodyClick?: (requestId: string, index: number) => void;
  onViewDetail?: (requestId: string) => void;
}

export const DEFAULT_HIDDEN_COLUMN_IDS = ['status', 'source', 'apiFormat', 'clientIP', 'tokensPerSecond'];

export const DEFAULT_MOBILE_HIDDEN_COLUMN_IDS = [...DEFAULT_HIDDEN_COLUMN_IDS, 'channel', 'cost', 'duration', 'caller'];

export const MODEL_ID_COLUMN = 'modelID' as const;

function getStringFilterValues(value: unknown): string[] {
  return Array.isArray(value) ? value.filter((item): item is string => typeof item === 'string') : [];
}

export function useRequestsColumns(options?: UseRequestsColumnsOptions): ColumnDef<Request>[] {
  const { t, i18n } = useTranslation();
  const locale = i18n.language === 'zh' ? zhCN : enUS;
  const permissions = useRequestPermissions();
  const { hasSystemScope } = usePermissions();
  const { data: settings } = useGeneralSettings();
  const { data: securitySettings } = useSecuritySettings();
  const updateSecuritySettings = useUpdateSecuritySettings();
  const { navigateWithSearch } = usePaginationSearch({ defaultPageSize: 20 });
  const canManageSecuritySettings = hasSystemScope('write_settings');

  const blockedIPs = securitySettings?.blockedIPs ?? [];
  const showIPBanIcon = securitySettings?.showRequestLogIPBanIcon === true;

  const normalizeBlockedIPs = (ips: string[]) => Array.from(new Set(ips.map((ip) => ip.trim()).filter((ip) => ip.length > 0)));

  const handleBlockIP = async (clientIP: string) => {
    const normalizedIP = clientIP.trim();
    if (!normalizedIP) return;

    const nextBlockedIPs = normalizeBlockedIPs([...blockedIPs, normalizedIP]);
    if (nextBlockedIPs.length === blockedIPs.length && blockedIPs.includes(normalizedIP)) {
      toast.info(t('requests.actions.ipAlreadyBlocked'));
      return;
    }

    await updateSecuritySettings.mutateAsync({ blockedIPs: nextBlockedIPs });
  };

  const handleUnblockIP = async (clientIP: string) => {
    const normalizedIP = clientIP.trim();
    if (!normalizedIP) return;

    await updateSecuritySettings.mutateAsync({ blockedIPs: blockedIPs.filter((ip) => ip.trim() !== normalizedIP) });
  };

  const openDetail = (requestId: string) => {
    if (options?.onViewDetail) {
      options.onViewDetail(requestId);
      return;
    }

    navigateWithSearch({
      to: '/project/requests/$requestId',
      params: { requestId },
    });
  };

  const columns: ColumnDef<Request>[] = [
    {
      id: 'request',
      accessorFn: (row) => row.createdAt,
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.request')} />,
      enableSorting: true,
      enableHiding: false,
      cell: ({ row }) => {
        const request = row.original;
        return (
          <div className='flex min-w-[142px] flex-col gap-1'>
            <button
              type='button'
              onClick={() => options?.onBodyClick?.(request.id, row.index)}
              className='text-left text-sm font-medium hover:underline'
            >
              {format(new Date(request.createdAt), 'yyyy-MM-dd HH:mm:ss', { locale })}
            </button>
            <Badge className={`${getStatusColor(request.status)} w-fit`}>{t(`requests.status.${request.status}`)}</Badge>
          </div>
        );
      },
    },
    {
      id: 'status',
      accessorKey: 'status',
      enableHiding: false,
      filterFn: (row, id, value) => {
        const values = getStringFilterValues(value);
        return values.includes(row.getValue(id));
      },
      cell: () => null,
    },
    {
      id: 'modelID',
      accessorFn: (row) => row.modelID,
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.model')} />,
      enableSorting: false,
      enableHiding: false,
      cell: ({ row }) => {
        const request = row.original;
        const executions = request.executions?.edges?.flatMap((edge) => (edge.node ? [edge.node] : [])) ?? [];
        const reasoningEffort = executions[0]?.reasoningEffort ?? request.reasoningEffort;
        const passThroughApplied = executions.some((execution) => execution.passThroughApplied);

        return (
          <div className='flex min-w-[160px] flex-col gap-1'>
            <span className='font-mono text-xs font-medium'>{request.modelID || t('requests.columns.unknown')}</span>
            <div className='flex items-center gap-1.5'>
              {reasoningEffort && (
                <Badge className='border-sky-200 bg-sky-100 text-sky-800 dark:border-sky-800 dark:bg-sky-900/20 dark:text-sky-300'>
                  {reasoningEffort}
                </Badge>
              )}
              <Tooltip>
                <TooltipTrigger asChild>
                  <span
                    className={`inline-flex h-5 w-5 items-center justify-center ${
                      passThroughApplied ? 'text-amber-700 dark:text-amber-300' : 'text-muted-foreground/45'
                    }`}
                    tabIndex={0}
                    role='img'
                    aria-label={t(passThroughApplied ? 'requests.tooltips.passThroughApplied' : 'requests.tooltips.passThroughNotApplied')}
                  >
                    <IconRoute className='h-3.5 w-3.5' />
                  </span>
                </TooltipTrigger>
                <TooltipContent>
                  {t(passThroughApplied ? 'requests.tooltips.passThroughApplied' : 'requests.tooltips.passThroughNotApplied')}
                </TooltipContent>
              </Tooltip>
            </div>
          </div>
        );
      },
    },
    {
      id: 'apiFormat',
      accessorFn: (row) => row.format,
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.apiFormat')} />,
      enableSorting: false,
      enableHiding: true,
      cell: ({ row }) => {
        const format = row.original.format;
        return format ? (
          <span className='inline-flex items-center rounded-md border border-zinc-200 bg-zinc-50 px-2 py-0.5 text-xs font-medium text-zinc-700 dark:border-zinc-700 dark:bg-zinc-800/50 dark:text-zinc-300'>
            {format}
          </span>
        ) : (
          <span className='text-muted-foreground text-xs'>-</span>
        );
      },
    },
    {
      id: 'passThrough',
      accessorFn: (row) => row.executions?.edges?.some((edge) => edge.node?.passThroughApplied) ?? false,
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.passThrough')} />,
      enableSorting: false,
      enableHiding: true,
      cell: ({ row }) => {
        const executions = row.original.executions?.edges?.map((edge) => edge.node).filter(Boolean) || [];
        const appliedExecution = executions.find((execution) => execution?.passThroughApplied);

        if (!appliedExecution) {
          return <div className='text-muted-foreground text-xs'>-</div>;
        }

        return (
          <Badge className='border-amber-200 bg-amber-100 text-amber-800 dark:border-amber-800 dark:bg-amber-900/20 dark:text-amber-300'>
            {t('requests.passThrough.applied')}
          </Badge>
        );
      },
    },
    {
      accessorKey: 'reasoningEffort',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.reasoningEffort')} />,
      enableSorting: false,
      enableHiding: true,
      cell: ({ row }) => {
        const latestExecution = row.original.executions?.edges?.[0]?.node;
        const reasoningEffort = latestExecution
          ? latestExecution.reasoningEffort
          : row.original.status === 'processing'
            ? undefined
            : row.original.reasoningEffort;

        if (!reasoningEffort) {
          return <div className='text-muted-foreground text-xs'>-</div>;
        }

        return (
          <Badge className='border-sky-200 bg-sky-100 text-sky-800 dark:border-sky-800 dark:bg-sky-900/20 dark:text-sky-300'>
            {reasoningEffort}
          </Badge>
        );
      },
    },

    {
      id: 'stream',
      accessorKey: 'stream',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.stream')} />,
      enableSorting: false,
      cell: ({ row }) => {
        const isStream = row.original.stream;
        return (
          <Badge
            className={
              isStream
                ? 'border-green-200 bg-green-100 text-green-800 dark:border-green-800 dark:bg-green-900/20 dark:text-green-300'
                : 'border-gray-200 bg-gray-100 text-gray-800 dark:border-gray-800 dark:bg-gray-900/20 dark:text-gray-300'
            }
          >
            {isStream ? t('requests.stream.streaming') : t('requests.stream.nonStreaming')}
          </Badge>
        );
      },
      filterFn: (row, _id, value) => {
        const values = getStringFilterValues(value);
        return values.includes(row.original.stream?.toString() || '-');
      },
      enableHiding: true,
    },
    {
      id: 'source',
      accessorKey: 'source',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.source')} />,
      enableSorting: false,
      cell: ({ row }) => {
        const source = row.getValue('source') as string;
        const sourceColors: Record<string, string> = {
          api: 'bg-blue-100 text-blue-800 border-blue-200 dark:bg-blue-900/20 dark:text-blue-300 dark:border-blue-800',
          playground: 'bg-purple-100 text-purple-800 border-purple-200 dark:bg-purple-900/20 dark:text-purple-300 dark:border-purple-800',
        };
        return (
          <Badge
            className={
              sourceColors[source] ||
              'border-gray-200 bg-gray-100 text-gray-800 dark:border-gray-800 dark:bg-gray-900/20 dark:text-gray-300'
            }
          >
            {t(`requests.source.${source}`)}
          </Badge>
        );
      },
      filterFn: (row, id, value) => {
        const values = getStringFilterValues(value);
        return values.includes(row.getValue(id));
      },
    },
    {
      id: 'clientIP',
      accessorKey: 'clientIP',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.clientIP')} />,
      enableSorting: false,
      enableHiding: true,
      cell: ({ row }) => {
        const normalizedIP = row.original.clientIP?.trim() ?? '';
        if (!normalizedIP) return <span className='text-muted-foreground text-xs'>-</span>;

        const isBlocked = blockedIPs.includes(normalizedIP);
        return (
          <div className='flex items-center gap-2'>
            <span className='font-mono text-xs'>{normalizedIP}</span>
            {canManageSecuritySettings &&
              showIPBanIcon &&
              (isBlocked ? (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon-sm'
                      className='h-6 w-6 shrink-0 text-red-500/80 hover:bg-red-50 hover:text-red-600 dark:text-red-300/80 dark:hover:bg-red-950/30 dark:hover:text-red-300'
                      disabled={updateSecuritySettings.isPending}
                      onClick={() => void handleUnblockIP(normalizedIP)}
                      aria-label={t('requests.actions.unblockIP')}
                    >
                      <Ban className='h-3.5 w-3.5' />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{t('requests.actions.unblockIP')}</TooltipContent>
                </Tooltip>
              ) : (
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon-sm'
                      className='text-muted-foreground h-6 w-6 shrink-0 hover:bg-red-50 hover:text-red-600 dark:hover:bg-red-950/30 dark:hover:text-red-300'
                      disabled={updateSecuritySettings.isPending}
                      onClick={() => void handleBlockIP(normalizedIP)}
                      aria-label={t('requests.actions.blockIP')}
                    >
                      <Ban className='h-3.5 w-3.5' />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{t('requests.actions.blockIP')}</TooltipContent>
                </Tooltip>
              ))}
          </div>
        );
      },
    },
    ...(permissions.canViewChannels
      ? ([
          {
            id: 'channel',
            accessorFn: (row) => row.executions?.edges?.[0]?.node?.channel?.id ?? row.channel?.id ?? '',
            header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.channel')} />,
            enableSorting: false,
            enableHiding: true,
            cell: ({ row }) => {
              const request = row.original;
              const executions = request.executions?.edges?.flatMap((edge) => (edge.node ? [edge.node] : [])) ?? [];
              const finalExecution = executions[0];
              const channel = finalExecution?.channel ?? request.channel;
              const hasExecutionPath =
                executions.length > 1 ||
                executions.some((execution) => execution.modelID && execution.modelID !== request.modelID) ||
                executions.some((execution) => execution.channel?.id && execution.channel.id !== channel?.id);

              if (!channel) return <span className='text-muted-foreground font-mono text-xs'>-</span>;

              return (
                <div className='flex min-w-[120px] items-center gap-1.5'>
                  <span className='font-mono text-xs'>{channel.name}</span>
                  {hasExecutionPath && (
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon-sm'
                          className='h-6 w-6 shrink-0 text-rose-600 hover:bg-rose-50 hover:text-rose-700 dark:text-rose-300 dark:hover:bg-rose-950/30 dark:hover:text-rose-200'
                          onClick={() => openDetail(request.id)}
                          aria-label={t('requests.tooltips.executionChain')}
                        >
                          <IconArrowsJoin2 className='h-3.5 w-3.5' />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent side='right' className='max-w-xs p-2'>
                        <div className='space-y-1.5'>
                          <p className='text-xs font-medium'>{t('requests.tooltips.executionChain')}</p>
                          {[...executions].reverse().map((execution, index) => (
                            <div key={execution.id ?? index} className='flex items-center gap-2 text-xs'>
                              <Badge className={`${getStatusColor(execution.status ?? '')} h-5 px-1.5 text-[10px]`}>
                                {execution.status ? t(`requests.status.${execution.status}`) : t('requests.columns.unknown')}
                              </Badge>
                              <span>{execution.channel?.name || t('requests.columns.unknown')}</span>
                            </div>
                          ))}
                        </div>
                      </TooltipContent>
                    </Tooltip>
                  )}
                </div>
              );
            },
            filterFn: (row, _id, value) => {
              // For client-side filtering, check if any of the selected channels match
              const values = getStringFilterValues(value);
              if (values.length === 0) return true; // No filter applied

              const channel = row.original.executions?.edges?.[0]?.node?.channel ?? row.original.channel;
              return !!channel?.id && values.includes(channel.id);
            },
          },
        ] as ColumnDef<Request>[])
      : []),
    // API Key column - only show if user has permission to view API keys
    ...(permissions.canViewApiKeys
      ? ([
          {
            accessorKey: 'apiKey',
            header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.apiKey')} />,
            enableSorting: false,
            cell: ({ row }) => {
              return <div className='font-mono text-xs'>{row.original.apiKey?.name || '-'}</div>;
            },
          },
        ] as ColumnDef<Request>[])
      : []),
    // User column - only show if user has permission to view users (admin scope)
    ...(permissions.canViewUsers
      ? ([
          {
            id: 'user',
            accessorFn: (row: Request) => row.apiKey?.user?.id ?? '',
            header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.user')} />,
            enableSorting: false,
            enableHiding: true,
            cell: ({ row }) => {
              const user = row.original.apiKey?.user;
              if (!user) return <div className='text-muted-foreground text-xs'>-</div>;
              const name = [user.firstName, user.lastName].filter(Boolean).join(' ').trim();
              return (
                <div className='text-xs'>
                  <div className='font-medium'>{name || user.email}</div>
                  {name && user.email && <div className='text-muted-foreground'>{user.email}</div>}
                </div>
              );
            },
          },
        ] as ColumnDef<Request>[])
      : [
          {
            id: 'user',
            header: () => null,
            cell: () => null,
            enableHiding: true,
            meta: { className: 'hidden' },
          } as ColumnDef<Request>,
        ]),

    {
      accessorKey: 'status',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.status')} />,
      cell: ({ row }) => {
        const status = row.getValue('status') as string;
        return <Badge className={getStatusColor(status)}>{t(`requests.status.${status}`)}</Badge>;
      },
      filterFn: (row, id, value) => {
        const values = getStringFilterValues(value);
        return values.includes(row.getValue(id));
      },
      enableSorting: false,
      enableHiding: true,
    },
    {
      id: 'usage',
      accessorFn: (row) => {
        const usageLog = row.usageLogs?.edges?.[0]?.node;
        return (usageLog?.promptTokens ?? 0) + (usageLog?.completionTokens ?? 0);
      },
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.usage')} />,
      enableSorting: true,
      enableHiding: false,
      cell: ({ row }) => {
        const usageLog = row.original.usageLogs?.edges?.[0]?.node;
        if (!usageLog) return <span className='text-muted-foreground text-xs'>-</span>;

        const promptTokens = usageLog.promptTokens ?? 0;
        const completionTokens = usageLog.completionTokens ?? 0;
        const readCacheTokens = usageLog.promptCachedTokens ?? 0;
        const writeCacheTokens = usageLog.promptWriteCachedTokens ?? 0;
        const hasCache = readCacheTokens > 0 || writeCacheTokens > 0;

        return (
          <div className='min-w-[170px] space-y-1 text-xs'>
            <div className='flex items-center gap-3 font-medium'>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className='inline-flex items-center gap-1' tabIndex={0} role='img' aria-label={t('requests.tooltips.inputTokens')}>
                    <ArrowUp className='text-muted-foreground h-3.5 w-3.5' />
                    {promptTokens.toLocaleString()}
                  </span>
                </TooltipTrigger>
                <TooltipContent>{t('requests.tooltips.inputTokens')}</TooltipContent>
              </Tooltip>
              <Tooltip>
                <TooltipTrigger asChild>
                  <span className='inline-flex items-center gap-1' tabIndex={0} role='img' aria-label={t('requests.tooltips.outputTokens')}>
                    <ArrowDown className='text-muted-foreground h-3.5 w-3.5' />
                    {completionTokens.toLocaleString()}
                  </span>
                </TooltipTrigger>
                <TooltipContent>{t('requests.tooltips.outputTokens')}</TooltipContent>
              </Tooltip>
            </div>
            <div className='text-muted-foreground whitespace-nowrap'>
              {hasCache
                ? `${t('requests.columns.cache')} ${readCacheTokens.toLocaleString()} (${t('requests.columns.read')})  ${writeCacheTokens.toLocaleString()} (${t('requests.columns.write')})`
                : `${t('requests.columns.cache')} -`}
            </div>
          </div>
        );
      },
    },
    {
      id: 'cost',
      accessorFn: (row) => row.usageLogs?.edges?.[0]?.node?.totalCost ?? null,
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.cost')} />,
      enableSorting: false,
      enableHiding: true,
      cell: ({ row }) => {
        const cost = row.original.usageLogs?.edges?.[0]?.node?.totalCost;
        if (cost == null) return <span className='font-mono text-xs'>-</span>;

        return (
          <span className='font-mono text-xs font-medium'>
            {t('currencies.format', {
              val: cost,
              currency: settings?.currencyCode ?? 'USD',
              locale: i18n.language === 'zh' ? 'zh-CN' : 'en-US',
              minimumFractionDigits: 6,
            })}
          </span>
        );
      },
    },
    {
      id: 'duration',
      accessorFn: (row) => row.metricsLatencyMs ?? null,
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.duration')} />,
      enableSorting: true,
      enableHiding: true,
      cell: ({ row }) => {
        const request = row.original;
        if (request.status !== 'completed' || request.metricsLatencyMs == null) {
          return <span className='text-muted-foreground text-xs'>-</span>;
        }

        if (!request.stream) {
          return (
            <span className='font-mono text-xs'>
              {t('requests.duration.total', { duration: formatDuration(request.metricsLatencyMs) })} · {t('requests.stream.nonStreaming')}
            </span>
          );
        }

        return (
          <div className='min-w-[128px] font-mono text-xs'>
            {request.metricsFirstTokenLatencyMs != null && (
              <div>{t('requests.duration.firstToken', { duration: formatDuration(request.metricsFirstTokenLatencyMs) })}</div>
            )}
            <div className='text-muted-foreground'>
              {t('requests.duration.total', { duration: formatDuration(request.metricsLatencyMs) })} · {t('requests.stream.streaming')}
            </div>
          </div>
        );
      },
      sortingFn: (rowA, rowB) => (rowA.original.metricsLatencyMs ?? 0) - (rowB.original.metricsLatencyMs ?? 0),
    },
    {
      id: 'tokensPerSecond',
      accessorFn: (row) => getTokensPerSecondValue(row) ?? 0,
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.tokensPerSecond')} />,
      enableSorting: true,
      enableHiding: true,
      cell: ({ row }) => <span className='font-mono text-xs'>{calculateTokensPerSecond(row.original)}</span>,
      sortingFn: (rowA, rowB) => (getTokensPerSecondValue(rowA.original) ?? 0) - (getTokensPerSecondValue(rowB.original) ?? 0),
    },
    {
      id: 'caller',
      accessorFn: (row) => row.apiKey?.id ?? '',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('requests.columns.caller')} />,
      enableSorting: false,
      enableHiding: true,
      cell: ({ row }) => {
        const request = row.original;
        if (request.source !== 'api') {
          return <Badge variant='secondary'>{t(`requests.source.${request.source}`)}</Badge>;
        }

        return <span className='font-mono text-xs'>{request.apiKey?.name || '-'}</span>;
      },
      filterFn: (row, _id, value) => {
        const values = getStringFilterValues(value);
        if (values.length === 0) return true;
        return values.includes(row.original.apiKey?.id ?? '');
      },
    },
    {
      id: 'details',
      header: () => <span className='sr-only'>{t('requests.columns.details')}</span>,
      cell: ({ row }) => (
        <Tooltip>
          <TooltipTrigger asChild>
            <Button
              type='button'
              variant='ghost'
              size='icon-sm'
              className='h-8 w-8'
              onClick={() => openDetail(row.original.id)}
              aria-label={t('requests.actions.viewDetails')}
            >
              <FileText className='h-4 w-4' />
            </Button>
          </TooltipTrigger>
          <TooltipContent>{t('requests.actions.viewDetails')}</TooltipContent>
        </Tooltip>
      ),
      enableHiding: false,
    },
  ];

  return columns;
}
