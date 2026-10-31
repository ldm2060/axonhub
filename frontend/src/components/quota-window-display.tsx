import { type ReactNode } from 'react';
import { useTranslation } from 'react-i18next';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { type ProviderQuotaLimit } from '@/features/system/data/quotas';
import { useGeneralSettings } from '@/features/system/data/system';

export function getDurationPercent(startAt?: string | null, endAt?: string | null): number | undefined {
  if (!startAt || !endAt) return undefined;
  const start = new Date(startAt).getTime();
  const end = new Date(endAt).getTime();
  if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) return undefined;
  return Math.max(0, Math.min(100, ((Date.now() - start) / (end - start)) * 100));
}

// Fraction of a limit's reset window that has elapsed, derived from the
// PeriodStart/NextResetAt pair the backend stamps on windowed limits.
export function getLimitDurationPercent(limit: ProviderQuotaLimit): number | undefined {
  return getDurationPercent(limit.periodStart, limit.nextResetAt);
}

// UsageTimeBar shows usage on a single progress bar with a small triangle below
// it marking how far the reset window has elapsed (time progress). Hovering
// reveals the detailed figures via tooltip, keeping the row compact.
export function UsageTimeBar({ usagePercent, durationPercent, tooltip }: { usagePercent: number; durationPercent?: number; tooltip: ReactNode }) {
  const clamped = Math.min(Math.max(usagePercent || 0, 0), 100);
  const markerLeft = durationPercent === undefined ? undefined : Math.min(Math.max(durationPercent, 0), 100);
  const u = clamped / 100;
  let severity = u;
  if (durationPercent !== undefined && durationPercent > 0) {
    const d = Math.max(durationPercent / 100, 0.01);
    severity = u * (u / d);
  }
  severity = Math.min(1, Math.max(0, severity));

  // Tailwind 500 colors approximation for a modern, theme-friendly gradient:
  // Green (142, 71%, 45%), Yellow (45, 93%, 47%), Red (0, 84%, 60%)
  let h: number;
  let s: number;
  let l: number;
  if (severity < 0.5) {
    const n = severity * 2; // 0 to 1
    h = 142 - n * (142 - 45);
    s = 71 + n * (93 - 71);
    l = 45 + n * (47 - 45);
  } else {
    const n = (severity - 0.5) * 2; // 0 to 1
    h = 45 - n * 45;
    s = 93 - n * (93 - 84);
    l = 47 + n * (60 - 47);
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <div className='relative cursor-default pb-1.5' tabIndex={0}>
          <div className='bg-muted/60 h-1.5 w-full overflow-hidden rounded-full'>
            <div
              className='h-full transition-all duration-500'
              style={{ width: `${clamped}%`, backgroundColor: `hsl(${Math.round(h)}, ${Math.round(s)}%, ${Math.round(l)}%)` }}
            />
          </div>
          {markerLeft !== undefined && (
            <div className='absolute top-2 -translate-x-1/2' style={{ left: `${markerLeft}%` }} aria-hidden>
              {/* upward triangle pointing at the bar, marking elapsed time */}
              <div className='border-b-muted-foreground h-0 w-0 border-x-[3px] border-b-[4px] border-x-transparent' />
            </div>
          )}
        </div>
      </TooltipTrigger>
      <TooltipContent side='top'>{tooltip}</TooltipContent>
    </Tooltip>
  );
}

export const WINDOW_LABEL_KEYS: Record<string, string> = {
  '5h': 'quota.window.5h',
  '7d': 'quota.window.7d',
  '30d': 'quota.window.30d',
  daily: 'quota.window.daily',
  weekly: 'quota.window.weekly',
  monthly: 'quota.window.monthly',
  payg: 'quota.label.token_usage',
  credits: 'quota.label.credits_remaining',
  overage: 'quota.label.overage_window',
  cycle: 'quota.label.subscription',
  kwh: 'quota.label.kwh_remaining',
};

// Resolved display label for a limit's window: known windows use their i18n
// key, the Codex primary/secondary aliases collapse to the generic usage
// label, and unknown window names are shown verbatim.
export function quotaWindowLabel(limit: Pick<ProviderQuotaLimit, 'window'>, t: (key: string) => string): string {
  const labelKey = limit.window ? WINDOW_LABEL_KEYS[limit.window] : undefined;
  if (labelKey) return t(labelKey);
  if (!limit.window || limit.window === 'primary' || limit.window === 'secondary') return t('quota.label.token_usage');
  return limit.window;
}

// periodStart only exists because the backend priced the window from usage
// logs: only limits with a pinned period start take part in the cost
// aggregation, and that pricing runs for no provider that reports the window's
// money itself (Synthetic regenerates its weekly pool in ticks, and its limits
// carry a regen time, not a window start). Its presence is therefore the
// documented marker of a log-derived figure; a limit without one shows
// provider-reported money.
function periodMoneyIsProviderReported(limit: Pick<ProviderQuotaLimit, 'periodStart'>): boolean {
  return limit.periodStart == null;
}

// PeriodQuotaEstimate prices each limit window: the backend sums what the
// channel cost during the window from AxonHub usage logs and divides by the
// usage ratio the provider reported, which yields what the whole window is
// worth — or the provider reports the window's money itself (Synthetic's
// weekly credits), in which case those figures are shown directly. Windows
// without a money figure are simply absent. The tooltip explains whichever
// source produced the displayed rows, so a channel that mixes both never
// suggests one explains the other's rows.
export function PeriodQuotaEstimate({ limits, currency }: { limits: ProviderQuotaLimit[]; currency?: string }) {
  const { t, i18n } = useTranslation();
  const { data: generalSettings } = useGeneralSettings();

  const priced = limits.filter((limit) => limit.periodQuota != null);
  if (priced.length === 0) return null;

  // The tooltip must describe the rows it hangs over: when some rows are
  // provider-reported and others log-derived, the shared dual-source wording
  // is chosen so neither reading is dropped.
  const sources = priced.map((limit) => (periodMoneyIsProviderReported(limit) ? 'provider' : 'log'));
  const hintKey =
    sources.includes('provider') && sources.includes('log')
      ? 'quota.label.period_quota_hint_mixed'
      : sources[0] === 'provider'
        ? 'quota.label.period_quota_hint_provider'
        : 'quota.label.period_quota_hint';

  const formatCurrency = (val: number) =>
    t('currencies.format', {
      val,
      currency: currency || generalSettings?.currencyCode || 'USD',
      locale: i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });

  return (
    <div className='border-border/60 mt-3 space-y-2 border-t border-dashed pt-3'>
      <Tooltip>
        <TooltipTrigger asChild>
          <span className='text-muted-foreground cursor-default text-xs font-medium'>{t('quota.label.period_quota')}</span>
        </TooltipTrigger>
        <TooltipContent side='top' className='max-w-[260px]'>
          {t(hintKey)}
        </TooltipContent>
      </Tooltip>

      {priced.map((limit, index) => {
        const label = quotaWindowLabel(limit, t);

        // A mark next to the window name identifies provider-reported rows
        // while a sibling row of the same channel is log-derived, so the two
        // readings stay separately attributable.
        const sourceMark =
          sources.includes('provider') && sources.includes('log') && periodMoneyIsProviderReported(limit)
            ? ` · ${t('quota.label.period_quota_provider_source')}`
            : '';

        return (
          <div key={`${limit.window ?? limit.type}-${index}`} className='flex items-center justify-between text-xs'>
            <span className='text-muted-foreground'>
              {label}
              {sourceMark}
            </span>
            <span className='text-foreground font-medium'>
              {t('quota.label.period_quota_value', {
                // The backend only estimates a quota once it has a cost, so
                // both figures are always available here.
                used: formatCurrency(limit.periodCost ?? 0),
                total: formatCurrency(limit.periodQuota as number),
              })}
            </span>
          </div>
        );
      })}
    </div>
  );
}

export function useFormatTimeToReset() {
  const { t } = useTranslation();

  return (resetAtOrSeconds?: string | number | null, usedPercent?: number, regenerates?: boolean | number) => {
    if (!resetAtOrSeconds) return '';

    let resetTimeMs: number;
    if (typeof resetAtOrSeconds === 'number') {
      resetTimeMs = Date.now() + resetAtOrSeconds * 1000;
    } else {
      resetTimeMs = new Date(resetAtOrSeconds).getTime();
    }

    if (usedPercent === 0) return t('quota.label.no_usage_yet');

    const now = Date.now();
    const diffMs = resetTimeMs - now;
    const isRegen = regenerates != null && regenerates !== false;
    const regenPct = typeof regenerates === 'number' ? Math.round(regenerates * 100) : null;
    if (diffMs < 0) return isRegen ? t('quota.label.regenerating_now') : t('quota.label.reset_now');

    const diffMins = Math.floor(diffMs / 60000);
    const diffHours = Math.floor(diffMins / 60);
    const diffDays = Math.floor(diffHours / 24);

    const d = t('quota.label.d');
    const h = t('quota.label.h');
    const m = t('quota.label.m');

    let timeStr: string;
    if (diffDays > 0) timeStr = `${diffDays}${d} ${diffHours % 24}${h}`;
    else if (diffHours > 0) timeStr = `${diffHours}${h} ${diffMins % 60}${m}`;
    else timeStr = `${diffMins}${m}`;

    if (isRegen && regenPct != null) {
      return t('quota.label.regenerates_pct_in_time', { percent: regenPct, time: timeStr });
    }
    return isRegen ? t('quota.label.regenerates_in_time', { time: timeStr }) : t('quota.label.resets_in_time', { time: timeStr });
  };
}

// QuotaWindowBars renders one usage bar per quota limit — the same per-window
// block the provider quota rows show. Every limit gets a row: windowed limits
// carry their i18n window label and an elapsed-time marker, and a windowless
// limit (custom/generic monitors) falls back to the generic usage label.
export function QuotaWindowBars({ limits }: { limits: ProviderQuotaLimit[] }) {
  const { t } = useTranslation();
  const formatTimeToReset = useFormatTimeToReset();

  if (limits.length === 0) return null;

  return (
    <div className='space-y-1.5'>
      {limits.map((limit, index) => {
        const label = quotaWindowLabel(limit, t);
        const usedPct = limit.usageRatio * 100;
        const durationPct = getLimitDurationPercent(limit);
        const resetText = formatTimeToReset(limit.nextResetAt);

        return (
          <div
            key={`${limit.window ?? limit.type}-${index}`}
            className={index > 0 ? 'border-border/60 space-y-1.5 border-t border-dashed pt-3' : 'space-y-1.5'}
          >
            <div className='flex items-center justify-between text-xs'>
              <span className='text-muted-foreground font-medium'>{label}</span>
              <span className='text-foreground font-medium'>{t('quota.label.percent_used', { percent: Math.round(usedPct) })}</span>
            </div>
            <UsageTimeBar
              usagePercent={usedPct}
              durationPercent={durationPct}
              tooltip={
                <div className='space-y-0.5'>
                  <div className='font-medium'>{label}</div>
                  <div>{t('quota.label.percent_used', { percent: Math.round(usedPct) })}</div>
                  {durationPct !== undefined && (
                    <div>
                      {t('quota.label.time_elapsed')}: {Math.round(durationPct)}%
                    </div>
                  )}
                  {resetText && <div>{resetText}</div>}
                </div>
              }
            />
          </div>
        );
      })}
    </div>
  );
}
