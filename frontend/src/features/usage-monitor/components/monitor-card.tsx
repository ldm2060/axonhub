import { formatDistanceToNow } from 'date-fns';
import { Pencil, Pause, Play, RefreshCw, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useUpdateUsageMonitorChannel, useRefreshUsageMonitorChannel } from '../data/usage-monitor';
import type { UsageMonitorChannel } from '../data/schema';
import { useUsageMonitorContext } from '../context/usage-monitor-context';
import { SharedFieldRenderer } from './shared-field-renderer';

function StatusBadge({ status }: { status: UsageMonitorChannel['status'] }) {
  const { t } = useTranslation();

  switch (status) {
    case 'active':
      return <Badge className="bg-green-100 text-green-800">{t('usageMonitor.status.active')}</Badge>;
    case 'paused':
      return <Badge variant="secondary">{t('usageMonitor.status.paused')}</Badge>;
    case 'error':
      return <Badge variant="destructive">{t('usageMonitor.status.error')}</Badge>;
    default:
      return null;
  }
}

function getBorderClass(status: UsageMonitorChannel['status']): string {
  switch (status) {
    case 'active':
      return 'border-l-green-500';
    case 'error':
      return 'border-l-red-500';
    case 'paused':
      return 'border-l-gray-400';
    default:
      return '';
  }
}

export function MonitorCard({ channel }: { channel: UsageMonitorChannel }) {
  const { t } = useTranslation();
  const { setOpen, setCurrentChannel } = useUsageMonitorContext();
  const updateMutation = useUpdateUsageMonitorChannel();
  const refreshMutation = useRefreshUsageMonitorChannel();

  const isPaused = channel.status === 'paused';
  const isRefreshing = refreshMutation.isPending;

  function handleEdit() {
    setCurrentChannel(channel);
    setOpen('edit');
  }

  function handleDelete() {
    setCurrentChannel(channel);
    setOpen('delete');
  }

  function handleTogglePause() {
    updateMutation.mutate({
      id: channel.id,
      input: { status: isPaused ? 'active' : 'paused' },
    });
  }

  function handleRefresh() {
    refreshMutation.mutate(channel.id);
  }

  const lastPollText = channel.lastPollAt
    ? `${t('usageMonitor.lastUpdated')} ${formatDistanceToNow(new Date(channel.lastPollAt), { addSuffix: true })}`
    : t('usageMonitor.noData');

  const pollIntervalSec = channel.pollInterval;
  const pollIntervalText =
    pollIntervalSec >= 60
      ? `${Math.floor(pollIntervalSec / 60)}m`
      : `${pollIntervalSec}s`;

  return (
    <div className={`rounded-lg border border-l-4 p-4 space-y-3 ${getBorderClass(channel.status)}`}>
      {/* Header */}
      <div className="flex items-center justify-between gap-2">
        <div className="font-semibold truncate">{channel.name}</div>
        <StatusBadge status={channel.status} />
      </div>

      {/* Sub-info */}
      <div className="flex items-center gap-2">
        <Badge variant="outline" className="text-xs">
          {t(`usageMonitor.source.${channel.source}`)}
        </Badge>
        {channel.source === 'template' && channel.providerType && (
          <Badge variant="outline" className="text-xs capitalize">
            {channel.providerType.replace(/_/g, ' ')}
          </Badge>
        )}
        <span className="text-xs text-muted-foreground">
          {t('usageMonitor.pollInterval')}: {pollIntervalText}
        </span>
      </div>

      {/* Parsed fields */}
      {channel.parsedData && channel.parsedData.length > 0 && (
        <SharedFieldRenderer
          fields={channel.parsedData}
          displayFields={channel.displayFields ?? undefined}
        />
      )}

      {/* Footer */}
      <div className="space-y-1.5">
        {channel.lastPollError && (
          <div className="rounded bg-red-500/10 px-2 py-1 text-xs text-red-500">
            {channel.lastPollError}
          </div>
        )}

        <div className="flex items-center justify-between gap-2">
          <span className="text-xs text-muted-foreground">{lastPollText}</span>

          <div className="flex items-center gap-0.5">
            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon-sm" onClick={handleEdit}>
                  <Pencil className="size-3.5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{t('usageMonitor.editChannel')}</TooltipContent>
            </Tooltip>

            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon-sm" onClick={handleTogglePause} disabled={updateMutation.isPending}>
                  {isPaused ? <Play className="size-3.5" /> : <Pause className="size-3.5" />}
                </Button>
              </TooltipTrigger>
              <TooltipContent>{isPaused ? t('usageMonitor.resumeChannel') : t('usageMonitor.pauseChannel')}</TooltipContent>
            </Tooltip>

            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon-sm" onClick={handleRefresh} disabled={isRefreshing}>
                  <RefreshCw className={`size-3.5 ${isRefreshing ? 'animate-spin' : ''}`} />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{t('usageMonitor.refreshNow')}</TooltipContent>
            </Tooltip>

            <Tooltip>
              <TooltipTrigger asChild>
                <Button variant="ghost" size="icon-sm" onClick={handleDelete} className="text-red-500 hover:text-red-600">
                  <Trash2 className="size-3.5" />
                </Button>
              </TooltipTrigger>
              <TooltipContent>{t('usageMonitor.deleteChannel')}</TooltipContent>
            </Tooltip>
          </div>
        </div>
      </div>
    </div>
  );
}
