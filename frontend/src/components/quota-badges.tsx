import { Loader2, RefreshCw, Battery, BatteryLow, BatteryMedium, BatteryFull, BatteryWarning } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Popover, PopoverContent, PopoverTrigger } from '@/components/ui/popover';
import { useQuotaChannels, type QuotaChannel } from '@/features/system/data/quotas';
import type { ParsedField } from '@/features/usage-monitor/data/schema';
import { useQuotaEnforcementSettings, type QuotaEnforcementMode } from '@/features/system/data/system';
import { SharedFieldRenderer } from '@/features/usage-monitor/components/shared-field-renderer';

const BADGE_COLOR_CLASSES: Record<string, string> = {
  green: 'bg-green-500/10 text-green-500 border-green-500/20 hover:bg-green-500/20',
  red: 'bg-red-500/10 text-red-500 border-red-500/20 hover:bg-red-500/20',
  amber: 'bg-amber-500/10 text-amber-500 border-amber-500/20 hover:bg-amber-500/20',
};

const STATUS_LABELS = {
  available: 'quota.status.available',
  warning: 'quota.status.warning',
  exhausted: 'quota.status.exhausted',
  unknown: 'quota.status.unknown',
} as const;

type BatteryLevel = 'full' | 'medium' | 'low' | 'empty' | 'warning';

function getBatteryIcon(level: BatteryLevel) {
  switch (level) {
    case 'full':
      return BatteryFull;
    case 'medium':
      return BatteryMedium;
    case 'low':
      return BatteryLow;
    case 'warning':
      return BatteryWarning;
    default:
      return Battery;
  }
}

function getBatteryLevel(percentage: number, status: string): BatteryLevel {
  if (status === 'exhausted') return 'warning';
  const remaining = 100 - percentage;
  if (remaining < 5) return 'empty';
  if (remaining < 20) return 'low';
  if (remaining < 80) return 'medium';
  return 'full';
}

/** Derive overall usage percentage from max(percent) across percentage-format fields. */
function getOverallPercentage(parsedData: ParsedField[]): number {
  let max = 0;
  for (const field of parsedData) {
    if ((field.format === 'percentage' || field.format === 'fraction') && field.percent != null) {
      max = Math.max(max, field.percent);
    }
  }
  return max;
}

function QuotaRow({ channel, enforcementMode }: { channel: QuotaChannel; enforcementMode?: QuotaEnforcementMode | null }) {
  const { t } = useTranslation();

  const status = channel.quotaStatus ?? 'unknown';
  const statusLabel = t(STATUS_LABELS[status]);

  const enforcementEffect =
    enforcementMode && (status === 'exhausted' || (status === 'warning' && enforcementMode === 'DE_PRIORITIZE'))
      ? enforcementMode === 'EXHAUSTED_ONLY'
        ? ('blocked' as const)
        : ('deprioritized' as const)
      : null;

  const percentage = getOverallPercentage(channel.parsedData);
  const batteryLevel = getBatteryLevel(percentage, status);
  const BatteryIcon = getBatteryIcon(batteryLevel);

  const displayName = channel.channelName ?? channel.name;

  return (
    <div className='space-y-3 border-b py-3 first:pt-1 last:border-0 last:pb-1'>
      <div className='flex items-center justify-between'>
        <div className='flex items-center gap-2'>
          <BatteryIcon
            className={`h-4 w-4 ${status === 'exhausted' ? 'text-red-500' : status === 'warning' ? 'text-yellow-500' : 'text-muted-foreground'}`}
          />
          <span className='text-foreground font-medium'>{displayName}</span>
        </div>
        <div className='flex items-center gap-1.5'>
          <Badge
            variant={
              status === 'available' ? 'outline' : status === 'warning' ? 'secondary' : status === 'exhausted' ? 'destructive' : 'outline'
            }
            className={status === 'available' ? BADGE_COLOR_CLASSES.green : ''}
          >
            {statusLabel}
          </Badge>
          {enforcementEffect && (
            <Badge variant='outline' className={BADGE_COLOR_CLASSES[enforcementEffect === 'blocked' ? 'red' : 'amber']}>
              {t(`quota.status.${enforcementEffect}`)}
            </Badge>
          )}
        </div>
      </div>

      {channel.lastPollError && (
        <div className='ml-6 rounded bg-red-500/10 p-2 text-xs break-words text-red-500'>
          <span className='font-medium'>{t('quota.label.error')}:</span> {channel.lastPollError}
        </div>
      )}

      {channel.parsedData && channel.parsedData.length > 0 && (
        <SharedFieldRenderer
          fields={channel.parsedData}
          displayFields={channel.displayFields}
        />
      )}
    </div>
  );
}

function QuotaBadgeTrigger({ channels }: { channels: QuotaChannel[] }) {
  const highestUsed = Math.max(
    ...channels.map((c) => getOverallPercentage(c.parsedData)),
    0,
  );

  const hasExhausted = channels.some((c) => c.quotaStatus === 'exhausted');
  const hasWarning = channels.some((c) => c.quotaStatus === 'warning');

  let level: BatteryLevel = 'full';
  if (hasExhausted) level = 'warning';
  else if (hasWarning) level = 'low';
  else level = getBatteryLevel(highestUsed, 'available');

  const BatteryIcon = getBatteryIcon(level);
  const isWarning = level === 'warning';
  const textColor = isWarning ? 'text-red-500' : level === 'low' ? 'text-yellow-500' : 'text-muted-foreground';

  return <BatteryIcon className={`h-5 w-5 ${textColor} transition-colors`} />;
}

export function QuotaBadges({ isRefreshing, onRefresh }: { isRefreshing: boolean; onRefresh: () => void }) {
  const { t } = useTranslation();
  const channels = useQuotaChannels();
  const { data: enforcementSettings } = useQuotaEnforcementSettings();
  const enforcementMode = enforcementSettings?.enabled ? enforcementSettings.mode : null;

  if (channels.length === 0) return null;

  return (
    <Popover>
      <PopoverTrigger asChild>
        <button type='button' className='hover:bg-muted relative rounded-md p-2 transition-colors'>
          <QuotaBadgeTrigger channels={channels} />
        </button>
      </PopoverTrigger>
      <PopoverContent className={channels.length > 4 ? 'w-full sm:w-[640px]' : 'w-full sm:w-80'} align='end'>
        <div className='space-y-1'>
          <div className='mb-2 flex items-center justify-between'>
            <div className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>{t('system.providerQuota.title')}</div>
            <button
              onClick={onRefresh}
              disabled={isRefreshing}
              className='text-muted-foreground hover:text-foreground transition-colors'
              aria-label={t('system.providerQuota.refresh.label')}
            >
              {isRefreshing ? <Loader2 className='h-4 w-4 animate-spin' /> : <RefreshCw className='h-4 w-4' />}
            </button>
          </div>
          <div
            className={`max-h-[60vh] overflow-y-auto pr-1 pl-1 ${channels.length > 4 ? 'grid grid-cols-1 gap-x-4 sm:grid-cols-2' : ''}`}
          >
            {channels.map((channel) => (
              <QuotaRow key={channel.id} channel={channel} enforcementMode={enforcementMode} />
            ))}
          </div>
        </div>
      </PopoverContent>
    </Popover>
  );
}
