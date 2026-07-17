import { useState, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useGeneralSettings } from '@/features/system/data/system';
import type { AnalyticsDimensionStat } from '../data/analytics';

function formatExactNumber(value: number): string {
  return Math.round(value).toLocaleString();
}

type Dimension = 'channel' | 'model' | 'apiKey' | 'user';

interface DimensionDetailTableProps {
  channelStats: AnalyticsDimensionStat[];
  modelStats: AnalyticsDimensionStat[];
  apiKeyStats: AnalyticsDimensionStat[];
  userStats: AnalyticsDimensionStat[];
  isLoading: boolean;
}

export function DimensionDetailTable({ channelStats, modelStats, apiKeyStats, userStats, isLoading }: DimensionDetailTableProps) {
  const { t, i18n } = useTranslation();
  const [dimension, setDimension] = useState<Dimension>('channel');
  const { data: generalSettings } = useGeneralSettings();

  const currencyCode = generalSettings?.currencyCode || 'USD';
  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';

  const formatCurrency = useCallback(
    (val: number) =>
      t('currencies.format', {
        val,
        currency: currencyCode,
        locale,
        minimumFractionDigits: 4,
        maximumFractionDigits: 4,
      }),
    [currencyCode, locale, t]
  );

  const dimensionData: Record<Dimension, AnalyticsDimensionStat[]> = {
    channel: channelStats,
    model: modelStats,
    apiKey: apiKeyStats,
    user: userStats,
  };

  const currentData = dimensionData[dimension] || [];

  if (isLoading) {
    return (
      <Card className='hover-card'>
        <CardHeader>
          <Skeleton className='h-6 w-[200px]' />
        </CardHeader>
        <CardContent>
          <Skeleton className='h-[300px] w-full' />
        </CardContent>
      </Card>
    );
  }

  return (
    <Card className='hover-card'>
      <CardHeader className='flex flex-row items-center justify-between'>
        <CardTitle>{t('analytics.table.title')}</CardTitle>
        <Tabs value={dimension} onValueChange={(v) => setDimension(v as Dimension)}>
          <TabsList className='h-8'>
            <TabsTrigger value='channel' className='text-xs'>
              {t('analytics.table.channel')}
            </TabsTrigger>
            <TabsTrigger value='model' className='text-xs'>
              {t('analytics.table.model')}
            </TabsTrigger>
            <TabsTrigger value='apiKey' className='text-xs'>
              {t('analytics.table.apiKey')}
            </TabsTrigger>
            <TabsTrigger value='user' className='text-xs'>
              {t('analytics.table.user')}
            </TabsTrigger>
          </TabsList>
        </Tabs>
      </CardHeader>
      <CardContent>
        {currentData.length === 0 ? (
          <div className='text-muted-foreground flex h-[200px] items-center justify-center text-sm'>{t('analytics.table.noData')}</div>
        ) : (
          <div className='overflow-x-auto'>
            <table className='w-full min-w-[800px] table-fixed caption-bottom text-sm'>
              <colgroup>
                <col className='w-[20%]' />
                <col className='w-[10%]' />
                <col className='w-[10%]' />
                <col className='w-[10%]' />
                <col className='w-[10%]' />
                <col className='w-[10%]' />
                <col className='w-[10%]' />
                <col className='w-[10%]' />
                <col className='w-[10%]' />
              </colgroup>
              <thead className='[&_tr]:border-b'>
                <tr className='hover:bg-muted/50 border-b transition-colors'>
                  <th className='bg-card text-muted-foreground sticky left-0 z-10 px-4 py-2 text-left text-xs font-medium'>
                    {t('analytics.table.name')}
                  </th>
                  <th className='text-muted-foreground px-4 py-2 text-right text-xs font-medium'>{t('analytics.table.totalTokens')}</th>
                  <th className='text-muted-foreground px-4 py-2 text-right text-xs font-medium'>{t('analytics.table.inputTokens')}</th>
                  <th className='text-muted-foreground px-4 py-2 text-right text-xs font-medium'>{t('analytics.table.cachedTokens')}</th>
                  <th className='text-muted-foreground px-4 py-2 text-right text-xs font-medium'>{t('analytics.table.uncachedTokens')}</th>
                  <th className='text-muted-foreground px-4 py-2 text-right text-xs font-medium'>{t('analytics.table.outputTokens')}</th>
                  <th className='text-muted-foreground px-4 py-2 text-right text-xs font-medium'>{t('analytics.table.cacheHitRate')}</th>
                  <th className='text-muted-foreground px-4 py-2 text-right text-xs font-medium'>{t('analytics.table.requests')}</th>
                  <th className='text-muted-foreground px-4 py-2 text-right text-xs font-medium'>{t('analytics.table.cost')}</th>
                </tr>
              </thead>
              <tbody className='[&_tr:last-child]:border-0'>
                {currentData.map((item) => (
                  <tr key={item.id} className='hover:bg-muted/50 border-b transition-colors'>
                    <td className='bg-card sticky left-0 z-10 truncate px-4 py-2 text-sm font-medium'>{item.name}</td>
                    <td className='px-4 py-2 text-right text-sm font-medium whitespace-nowrap'>{formatExactNumber(item.totalTokens)}</td>
                    <td className='px-4 py-2 text-right text-sm whitespace-nowrap'>{formatExactNumber(item.inputTokens)}</td>
                    <td className='px-4 py-2 text-right text-sm whitespace-nowrap'>{formatExactNumber(item.cachedInputTokens)}</td>
                    <td className='px-4 py-2 text-right text-sm whitespace-nowrap'>
                      {formatExactNumber(item.inputTokens - item.cachedInputTokens)}
                    </td>
                    <td className='px-4 py-2 text-right text-sm whitespace-nowrap'>{formatExactNumber(item.outputTokens)}</td>
                    <td className='px-4 py-2 text-right text-sm whitespace-nowrap'>
                      {item.inputTokens > 0 ? ((item.cachedInputTokens / item.inputTokens) * 100).toFixed(1) : '0'}%
                    </td>
                    <td className='px-4 py-2 text-right text-sm whitespace-nowrap'>{formatExactNumber(item.requestCount)}</td>
                    <td className='px-4 py-2 text-right text-sm whitespace-nowrap'>{formatCurrency(item.cost)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
