'use client';

import { useMemo } from 'react';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { CartesianGrid, Area, AreaChart, ResponsiveContainer, Tooltip, XAxis, YAxis, type TooltipProps } from 'recharts';
import { formatNumber } from '@/utils/format-number';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { useGeneralSettings } from '../../system/data/system';
import { useDailyUsageStatsByUser, type DashboardMode } from '../data/dashboard';
import { ChartLegend } from './chart-legend';

const COLORS = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)', 'var(--chart-5)', 'var(--chart-6)'];

const WEEK_DAYS = 7;
const TOP_USERS = 10;

interface ChartRow {
  name: string;
  [key: string]: string | number;
}

export function WeeklyUsageLineChart({ mode }: { mode: DashboardMode }) {
  const { t, i18n } = useTranslation();
  const { data, isLoading, isFetching, error } = useDailyUsageStatsByUser(WEEK_DAYS, mode);
  const { data: generalSettings } = useGeneralSettings();

  const currencyCode = generalSettings?.currencyCode || 'USD';
  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';

  const formatCurrency = (val: number) =>
    t('currencies.format', {
      val,
      currency: currencyCode,
      locale,
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });

  const { chartData, users } = useMemo(() => {
    if (!data || data.length === 0) return { chartData: [] as ChartRow[], users: [] as { name: string; total: number }[] };

    const top = data.slice(0, TOP_USERS);

    // Collect the ordered set of dates from the first user's daily array
    // (the backend zero-fills every user to the same date range).
    const dates = top[0]?.daily.map((d) => d.date) ?? [];

    const rows: ChartRow[] = dates.map((dateStr) => {
      const [year, month, day] = dateStr.split('-').map(Number);
      const date = new Date(Date.UTC(year, month - 1, day));
      const row: ChartRow = {
        name: date.toLocaleDateString(locale, { month: '2-digit', day: '2-digit', timeZone: 'UTC' }),
      };
      return row;
    });

    top.forEach((user) => {
      user.daily.forEach((d, idx) => {
        if (rows[idx]) rows[idx][user.userName] = d.count;
      });
    });

    const userTotals = top.map((user) => ({
      name: user.userName,
      total: user.daily.reduce((s, d) => s + d.count, 0),
    }));

    return { chartData: rows, users: userTotals };
  }, [data, locale]);

  const maxValue = useMemo(
    () => Math.max(...chartData.flatMap((row) => users.map((u) => Number(row[u.name] ?? 0))), 0),
    [chartData, users]
  );
  const yDomain = [0, Math.max(10, Math.ceil(maxValue * 1.1))] as [number, number];

  const legendItems = users.map((u, index) => ({
    name: u.name,
    index: index + 1,
    color: COLORS[index % COLORS.length],
    primaryValue: formatNumber(u.total),
  }));

  type Payload = {
    name?: string;
    value?: number;
    color?: string;
    payload?: ChartRow;
  };

  type CombinedTooltipProps = TooltipProps<number, string> & {
    payload?: Payload[];
  };

  const tooltipContent = (props: CombinedTooltipProps) => {
    if (!props.active) return null;
    const row = props.payload?.[0]?.payload;
    const dateLabel = row?.name != null ? String(row.name) : String(props.label ?? '');
    if (!dateLabel) return null;

    // Pull token/cost for the hovered day from the raw data (top users only).
    const dayEntries = (data ?? [])
      .slice(0, TOP_USERS)
      .map((user) => {
        const idx = user.daily.findIndex((d) => {
          const [y, m, d2] = d.date.split('-').map(Number);
          return (
            dateLabel ===
            new Date(Date.UTC(y, m - 1, d2)).toLocaleDateString(locale, {
              month: '2-digit',
              day: '2-digit',
              timeZone: 'UTC',
            })
          );
        });
        if (idx < 0) return null;
        const stat = user.daily[idx];
        return { name: user.userName, stat };
      })
      .filter((e): e is { name: string; stat: { count: number; tokens: number; cost: number } } => e !== null)
      .sort((a, b) => b.stat.count - a.stat.count);

    return (
      <div className='bg-background/90 rounded-md border px-3 py-2 text-xs shadow-sm backdrop-blur'>
        <div className='text-foreground mb-1 text-sm font-medium'>{dateLabel}</div>
        <div className='max-h-48 space-y-1 overflow-auto'>
          {dayEntries.length === 0 ? (
            <div className='text-muted-foreground'>{t('dashboard.charts.noUserData')}</div>
          ) : (
            dayEntries.map((e) => (
              <div key={e.name} className='flex justify-between gap-4'>
                <span className='text-muted-foreground'>{e.name}</span>
                <span className='font-medium tabular-nums'>
                  {formatNumber(e.stat.count)} · {formatNumber(e.stat.tokens)} · {formatCurrency(e.stat.cost)}
                </span>
              </div>
            ))
          )}
        </div>
      </div>
    );
  };

  return (
    <Card className='hover-card'>
      <CardHeader>
        <CardTitle>{t('dashboard.charts.weeklyUsageTrend')}</CardTitle>
        <CardDescription>{t('dashboard.charts.weeklyUsageTrendDescription')}</CardDescription>
      </CardHeader>
      <CardContent>
        <div className='relative space-y-6'>
          {isLoading ? (
            <div className='flex h-[300px] items-center justify-center'>
              <Skeleton className='h-[250px] w-full rounded-md' />
            </div>
          ) : error ? (
            <div className='flex h-[300px] items-center justify-center'>
              <div className='text-sm text-red-500'>
                {t('dashboard.charts.errorLoadingChart')} {error.message}
              </div>
            </div>
          ) : chartData.length === 0 || users.length === 0 ? (
            <div className='flex h-[300px] items-center justify-center'>
              <div className='text-muted-foreground text-sm'>{t('dashboard.charts.noUserData')}</div>
            </div>
          ) : (
            <>
              <ResponsiveContainer width='100%' height={350}>
                <AreaChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
                  <defs>
                    {users.map((u, index) => (
                      <linearGradient key={`${u.name}-fill`} id={`weeklyUsage-${index}`} x1='0' y1='0' x2='0' y2='1'>
                        <stop offset='5%' stopColor={COLORS[index % COLORS.length]} stopOpacity={0.3} />
                        <stop offset='95%' stopColor={COLORS[index % COLORS.length]} stopOpacity={0} />
                      </linearGradient>
                    ))}
                  </defs>
                  <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' vertical={false} />
                  <XAxis dataKey='name' stroke='var(--muted-foreground)' fontSize={12} tickLine axisLine padding={{ right: 24 }} />
                  <YAxis
                    stroke='var(--muted-foreground)'
                    fontSize={12}
                    tickLine
                    axisLine
                    domain={yDomain}
                    tickFormatter={(value) => formatNumber(value)}
                    width={40}
                    tickMargin={8}
                    tickCount={6}
                  />
                  <Tooltip content={tooltipContent} cursor={{ stroke: 'var(--muted)' }} isAnimationActive={false} />
                  {users.map((u, index) => (
                    <Area
                      key={u.name}
                      type='monotone'
                      dataKey={u.name}
                      stroke={COLORS[index % COLORS.length]}
                      strokeWidth={2}
                      fill={`url(#weeklyUsage-${index})`}
                      fillOpacity={1}
                      dot={{ r: 2.5, strokeWidth: 0, fill: COLORS[index % COLORS.length] }}
                      activeDot={{ r: 5, strokeWidth: 2, stroke: 'var(--background)' }}
                      isAnimationActive={false}
                    />
                  ))}
                </AreaChart>
              </ResponsiveContainer>
              <ChartLegend items={legendItems} />
            </>
          )}
          {isFetching && !isLoading && (
            <div className='bg-background/50 absolute inset-0 flex items-center justify-center'>
              <Loader2 className='text-muted-foreground h-6 w-6 animate-spin' />
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
