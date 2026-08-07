import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import type { UsageMonitorBindingSummary } from '../data/schema';
import { useUsageMonitorBindingSummaries } from '../data/usage-monitor';

function StrategyBadge({ strategy }: { strategy: UsageMonitorBindingSummary['strategy'] }) {
  const { t } = useTranslation();
  return (
    <Badge variant='outline' className='text-xs'>
      {strategy === 'any' ? t('usageMonitor.bindingSummary.strategyAny') : t('usageMonitor.bindingSummary.strategyAll')}
    </Badge>
  );
}

function MatchedBadge({ matched }: { matched: boolean }) {
  const { t } = useTranslation();
  return (
    <Badge className={`text-xs ${matched ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'}`}>
      {matched ? t('usageMonitor.bindingSummary.matched') : t('usageMonitor.bindingSummary.ready')}
    </Badge>
  );
}

export function MonitorBindingSummary({ monitorID }: { monitorID: string }) {
  const { t } = useTranslation();
  const { data: summaries, isLoading } = useUsageMonitorBindingSummaries();

  if (isLoading) {
    return null;
  }

  const bindings = (summaries ?? []).filter((s) => s.usageMonitorChannelID === monitorID);

  if (bindings.length === 0) {
    return <p className='text-muted-foreground text-xs'>{t('usageMonitor.bindingSummary.none')}</p>;
  }

  return (
    <div className='space-y-2'>
      <div className='text-muted-foreground text-xs font-medium'>{t('usageMonitor.bindingSummary.title')}</div>
      {bindings.map((binding) => (
        <div key={`${binding.channelID}-${binding.usageMonitorChannelID}`} className='bg-muted/30 space-y-1.5 rounded-md border px-3 py-2'>
          {/* Channel name + badges row */}
          <div className='flex flex-wrap items-center gap-1.5'>
            <span className='max-w-[180px] truncate text-sm font-medium'>{binding.channelName}</span>
            <StrategyBadge strategy={binding.strategy} />
            <MatchedBadge matched={binding.matched} />
          </div>

          {/* Status rule */}
          {binding.triggerStatuses.length > 0 && (
            <div className='flex items-center gap-1.5'>
              <span className='text-muted-foreground text-xs'>{t('usageMonitor.bindingSummary.statusRule')}:</span>
              <div className='flex items-center gap-1'>
                {binding.triggerStatuses.map((status) => (
                  <Badge key={status} variant='secondary' className='text-xs'>
                    {t(`usageMonitor.bindingSummary.triggerStatus.${status}`)}
                  </Badge>
                ))}
              </div>
            </div>
          )}

          {/* Condition rules */}
          {binding.conditions.length > 0 && (
            <div className='space-y-1'>
              <span className='text-muted-foreground text-xs'>{t('usageMonitor.bindingSummary.conditionRule')}:</span>
              {binding.conditions.map((cond, idx) => (
                <div key={`${cond.field}-${cond.operator}-${idx}`} className='bg-muted/50 rounded px-1.5 py-0.5 font-mono text-xs'>
                  {cond.field} {cond.operator} {cond.value}
                </div>
              ))}
            </div>
          )}

          {/* Reason */}
          {binding.reason && <div className='text-muted-foreground text-xs italic'>{binding.reason}</div>}
        </div>
      ))}
    </div>
  );
}
