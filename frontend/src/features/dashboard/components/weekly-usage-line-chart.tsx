'use client';

import { useCallback, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import {
  CartesianGrid,
  Legend,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts';
import { Loader2 } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { formatNumber } from '@/utils/format-number';
import { useGeneralSettings } from '../../system/data/system';
import { useDailyRequestStats, type DashboardMode } from '../data/dashboard';

interface WeeklyUsageLineChartProps {
  mode: DashboardMode;
}

const WEEK_DAYS = 7;

export function WeeklyUsageLineChart({ mode }: WeeklyUsageLineChartProps) {
  const { t, i18n } = useTranslation();
  const { data, isLoading, isFetching, error } = useDailyRequestStats(mode);
  const { data: generalSettings } = useGeneralSettings();

  const currencyCode = generalSettings?.currencyCode || 'USD';
  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';

  const formatCurrency = useCallback(
    (val: number, fractionDigits: number) =>
      t('currencies.format', {
        val,
        currency: currencyCode,
        locale,
        minimumFractionDigits: fractionDigits,
        maximumFractionDigits: fractionDigits,
      }),
    [currencyCode, locale, t]
  );

  const formatCostTick = useCallback(
    (value: number | string) => formatCurrency(Number(value), 0),
    [formatCurrency]
  );

  const tooltipFormatter = useCallback(
    (value: number | string, name: string) => {
      if (name === t('dashboard.stats.totalCost')) {
        return [formatCurrency(Number(value), 2), name];
      }
      return [formatNumber(Number(value)), name];
    },
    [formatCurrency, t]
  );

  const chartData = useMemo(() => {
    if (!data || data.length === 0) return [];
    const recent = data.slice(-WEEK_DAYS);
    return recent.map((stat) => {
      const [year, month, day] = stat.date.split('-').map(Number);
      const date = new Date(Date.UTC(year, month - 1, day));
      return {
        name: date.toLocaleDateString(locale, {
          month: '2-digit',
          day: '2-digit',
          timeZone: 'UTC',
        }),
        requests: stat.count,
        tokens: stat.tokens,
        cost: stat.cost,
      };
    });
  }, [data, locale]);

  const domains = useMemo(() => {
    const maxRequests = Math.max(...chartData.map((d) => d.requests), 0);
    const maxTokens = Math.max(...chartData.map((d) => d.tokens), 0);
    const maxCost = Math.max(...chartData.map((d) => d.cost), 0);
    return {
      requests: [0, Math.max(10, Math.ceil(maxRequests * 1.1))] as [number, number],
      tokens: [0, Math.max(1000, Math.ceil(maxTokens * 1.1))] as [number, number],
      cost: [0, Math.max(0.1, maxCost * 1.1)] as [number, number],
    };
  }, [chartData]);

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
          ) : chartData.length === 0 ? (
            <div className='flex h-[300px] items-center justify-center'>
              <div className='text-muted-foreground text-sm'>{t('dashboard.charts.noUserData')}</div>
            </div>
          ) : (
            <ResponsiveContainer width='100%' height={300}>
              <LineChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
                <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' vertical={false} />
                <XAxis
                  dataKey='name'
                  stroke='var(--muted-foreground)'
                  fontSize={12}
                  tickLine
                  axisLine
                />
                <YAxis
                  yAxisId='requests'
                  stroke='var(--chart-1)'
                  fontSize={12}
                  tickLine
                  axisLine
                  domain={domains.requests}
                  tickFormatter={(value) => formatNumber(value)}
                  width={40}
                  tickMargin={8}
                />
                <YAxis
                  yAxisId='tokens'
                  orientation='right'
                  stroke='var(--chart-2)'
                  fontSize={12}
                  tickLine
                  axisLine
                  domain={domains.tokens}
                  tickFormatter={(value) => formatNumber(value)}
                  width={40}
                  tickMargin={8}
                />
                <YAxis
                  yAxisId='cost'
                  orientation='right'
                  stroke='var(--chart-3)'
                  fontSize={12}
                  tickLine
                  axisLine
                  domain={domains.cost}
                  tickFormatter={formatCostTick}
                  width={60}
                  tickMargin={8}
                />
                <Tooltip
                  formatter={tooltipFormatter}
                  contentStyle={{
                    backgroundColor: 'var(--background)',
                    borderColor: 'var(--border)',
                    borderRadius: 'var(--radius)',
                    fontSize: '12px',
                  }}
                  itemStyle={{ padding: '2px 0' }}
                />
                <Legend verticalAlign='top' height={36} />
                <Line
                  yAxisId='requests'
                  type='monotone'
                  dataKey='requests'
                  name={t('dashboard.stats.requests')}
                  stroke='var(--chart-1)'
                  strokeWidth={2}
                  dot={{ r: 3 }}
                  activeDot={{ r: 5 }}
                  isAnimationActive={false}
                />
                <Line
                  yAxisId='tokens'
                  type='monotone'
                  dataKey='tokens'
                  name={t('dashboard.stats.totalTokens')}
                  stroke='var(--chart-2)'
                  strokeWidth={2}
                  dot={{ r: 3 }}
                  activeDot={{ r: 4 }}
                  isAnimationActive={false}
                />
                <Line
                  yAxisId='cost'
                  type='monotone'
                  dataKey='cost'
                  name={t('dashboard.stats.totalCost')}
                  stroke='var(--chart-3)'
                  strokeWidth={2}
                  dot={{ r: 3 }}
                  activeDot={{ r: 4 }}
                  isAnimationActive={false}
                />
              </LineChart>
            </ResponsiveContainer>
          )}
          {isFetching && !isLoading && (
            <div className='absolute inset-0 flex items-center justify-center bg-background/50'>
              <Loader2 className='h-6 w-6 animate-spin text-muted-foreground' />
            </div>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
