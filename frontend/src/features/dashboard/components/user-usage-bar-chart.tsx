import { useMemo } from 'react';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Bar, BarChart, CartesianGrid, Cell, ResponsiveContainer, Tooltip, XAxis, YAxis, type TooltipContentProps } from 'recharts';
import { formatNumber } from '@/utils/format-number';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { type TimePeriod } from '@/components/time-period-selector';
import { useGeneralSettings } from '../../system/data/system';
import { useUsageStatsByUser, type DashboardMode } from '../data/dashboard';
import { ChartLegend } from './chart-legend';

const COLORS = ['var(--chart-1)', 'var(--chart-2)', 'var(--chart-3)', 'var(--chart-4)', 'var(--chart-5)', 'var(--chart-6)'];

export type UserUsageMetric = 'requests' | 'tokens' | 'cost';

interface UserUsageBarChartProps {
  timePeriod: TimePeriod;
  metric: UserUsageMetric;
  mode: DashboardMode;
}

const DATA_KEY: Record<UserUsageMetric, 'requestCount' | 'totalTokens' | 'totalCost'> = {
  requests: 'requestCount',
  tokens: 'totalTokens',
  cost: 'totalCost',
};

function formatCompactNumber(value: number): string {
  if (value === 0) return '0';
  if (Math.abs(value) >= 1_000_000) return `${(value / 1_000_000).toFixed(1).replace(/\.0$/, '')}M`;
  if (Math.abs(value) >= 1_000) return `${(value / 1_000).toFixed(1).replace(/\.0$/, '')}k`;
  return `${value}`;
}

export function UserUsageBarChart({ timePeriod, metric, mode }: UserUsageBarChartProps) {
  const { t, i18n } = useTranslation();

  const { data, isLoading, isFetching, error } = useUsageStatsByUser(timePeriod, mode);
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

  const dataKey = DATA_KEY[metric];

  const { chartData } = useMemo(() => {
    if (!data) return { chartData: [] };
    const sorted = [...data].sort((a, b) => b[dataKey] - a[dataKey]);
    const top10 = sorted.slice(0, 10);
    return { chartData: top10 };
  }, [data, dataKey]);

  const formatMetricValue = (val: number) => {
    if (metric === 'cost') return formatCurrency(val);
    return formatNumber(val);
  };

  const formatYAxis = (value: number) => {
    if (value === 0) return '0';
    if (metric === 'tokens') {
      const mValue = value / 1_000_000;
      const formatted = mValue.toFixed(3).replace(/\.0+$|(?<=\.\d*[1-9])0+$/g, '');
      return `${formatted}M`;
    }
    return formatCompactNumber(value);
  };

  const tooltipContent = (props: TooltipContentProps) => {
    if (!props.active || !props.payload?.length) return null;
    const d = props.payload[0].payload;
    if (!d) return null;
    const tokenTotal = chartData.reduce((s, i) => s + i.totalTokens, 0);
    const costTotal = chartData.reduce((s, i) => s + i.totalCost, 0);
    const reqTotal = chartData.reduce((s, i) => s + i.requestCount, 0);

    return (
      <div className='bg-background/90 rounded-md border px-3 py-2 text-xs shadow-sm backdrop-blur'>
        <div className='text-foreground mb-1 text-sm font-medium'>{d.userName}</div>
        <div className='space-y-1'>
          <div className='flex justify-between gap-4'>
            <span className='text-muted-foreground'>{t('dashboard.stats.requestCount')}:</span>
            <span className='font-medium'>
              {formatNumber(d.requestCount)} ({reqTotal ? ((d.requestCount / reqTotal) * 100).toFixed(0) : 0}%)
            </span>
          </div>
          <div className='flex justify-between gap-4'>
            <span className='text-muted-foreground'>{t('dashboard.stats.tokenCount')}:</span>
            <span className='font-medium'>
              {formatNumber(d.totalTokens)} ({tokenTotal ? ((d.totalTokens / tokenTotal) * 100).toFixed(0) : 0}%)
            </span>
          </div>
          <div className='flex justify-between gap-4'>
            <span className='text-muted-foreground'>{t('dashboard.stats.userCost')}:</span>
            <span className='font-medium'>
              {formatCurrency(d.totalCost)} ({costTotal ? ((d.totalCost / costTotal) * 100).toFixed(0) : 0}%)
            </span>
          </div>
        </div>
      </div>
    );
  };

  const legendItems = chartData.map((item, index) => ({
    name: item.userName,
    index: index + 1,
    color: COLORS[index % COLORS.length],
    primaryValue: formatMetricValue(item[dataKey]),
  }));

  const titleKey = `dashboard.charts.${metric}ByUser`;
  const descKey = `dashboard.charts.${metric}ByUserDescription`;

  return (
    <Card className='hover-card'>
      <CardHeader>
        <CardTitle>{t(titleKey)}</CardTitle>
        <CardDescription>{t(descKey)}</CardDescription>
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
          ) : chartData.length === 0 ? (
            <div className='flex h-[300px] items-center justify-center'>
              <div className='text-muted-foreground text-sm'>{t('dashboard.charts.noUserData')}</div>
            </div>
          ) : (
            <>
              <ResponsiveContainer width='100%' height={300}>
                <BarChart data={chartData} barSize={32}>
                  <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' vertical={false} />
                  <XAxis dataKey='userName' hide />
                  <YAxis
                    tickLine={false}
                    axisLine={false}
                    width={60}
                    tick={{ fontSize: 12, fill: 'var(--muted-foreground)' }}
                    tickFormatter={formatYAxis}
                  />
                  <Tooltip content={tooltipContent} cursor={{ fill: 'var(--muted)' }} />
                  <Bar dataKey={dataKey} radius={[6, 6, 0, 0]} isAnimationActive={false}>
                    {chartData.map((_, index) => (
                      <Cell key={`cell-${index}`} fill={COLORS[index % COLORS.length]} />
                    ))}
                  </Bar>
                </BarChart>
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
