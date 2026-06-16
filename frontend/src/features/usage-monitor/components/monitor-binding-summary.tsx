import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { useUsageMonitorBindingSummaries } from '../data/usage-monitor';
import type { UsageMonitorBindingSummary } from '../data/schema';

function StrategyBadge({ strategy }: { strategy: UsageMonitorBindingSummary['strategy'] }) {
  const { t } = useTranslation();
  return (
    <Badge variant="outline" className="text-xs">
      {strategy === 'any'
        ? t('usageMonitor.bindingSummary.strategyAny')
        : t('usageMonitor.bindingSummary.strategyAll')}
    </Badge>
  );
}

function MatchedBadge({ matched }: { matched: boolean }) {
  const { t } = useTranslation();
  return (
    <Badge className={`text-xs ${matched ? 'bg-green-100 text-green-800' : 'bg-gray-100 text-gray-600'}`}>
      {matched
        ? t('usageMonitor.bindingSummary.matched')
        : t('usageMonitor.bindingSummary.ready')}
    </Badge>
  );
}

export function MonitorBindingSummary({ monitorID }: { monitorID: string }) {
  const { t } = useTranslation();
  const { data: summaries, isLoading } = useUsageMonitorBindingSummaries();

  if (isLoading) {
    return null;
  }

  const bindings = (summaries ?? []).filter(
    (s) => s.usageMonitorChannelID === monitorID,
  );

  if (bindings.length === 0) {
    return (
      <p className="text-xs text-muted-foreground">
        {t('usageMonitor.bindingSummary.none')}
      </p>
    );
  }

  return (
    <div className="space-y-2">
      <div className="text-xs font-medium text-muted-foreground">
        {t('usageMonitor.bindingSummary.title')}
      </div>
      {bindings.map((binding) => (
        <div
          key={`${binding.channelID}-${binding.usageMonitorChannelID}`}
          className="rounded-md border bg-muted/30 px-3 py-2 space-y-1.5"
        >
          {/* Channel name + badges row */}
          <div className="flex items-center gap-1.5 flex-wrap">
            <span className="text-sm font-medium truncate max-w-[180px]">
              {binding.channelName}
            </span>
            <StrategyBadge strategy={binding.strategy} />
            <MatchedBadge matched={binding.matched} />
          </div>

          {/* Status rule */}
          {binding.triggerStatuses.length > 0 && (
            <div className="flex items-center gap-1.5">
              <span className="text-xs text-muted-foreground">
                {t('usageMonitor.bindingSummary.statusRule')}:
              </span>
              <div className="flex items-center gap-1">
                {binding.triggerStatuses.map((status) => (
                  <Badge key={status} variant="secondary" className="text-xs">
                    {status}
                  </Badge>
                ))}
              </div>
            </div>
          )}

          {/* Condition rules */}
          {binding.conditions.length > 0 && (
            <div className="space-y-1">
              <span className="text-xs text-muted-foreground">
                {t('usageMonitor.bindingSummary.conditionRule')}:
              </span>
              {binding.conditions.map((cond, idx) => (
                <div
                  key={`${cond.field}-${cond.operator}-${idx}`}
                  className="text-xs font-mono bg-muted/50 rounded px-1.5 py-0.5"
                >
                  {cond.field} {cond.operator} {cond.value}
                </div>
              ))}
            </div>
          )}

          {/* Reason */}
          {binding.reason && (
            <div className="text-xs text-muted-foreground italic">
              {binding.reason}
            </div>
          )}
        </div>
      ))}
    </div>
  );
}
