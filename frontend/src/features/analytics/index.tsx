import { useNavigate } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { useAnalyticsFilterStore } from '@/stores/analyticsStore';
import { Button } from '@/components/ui/button';
import { Header } from '@/components/layout/header';
import { useGeneralSettings } from '@/features/system/data/system';
import { AnalyticsFilterBar } from './components/analytics-filter-bar';
import { CombinedTrendChart } from './components/combined-trend-chart';
import { DimensionDetailTable } from './components/dimension-detail-table';
import { DimensionPieCharts } from './components/dimension-pie-charts';
import { OverviewCards } from './components/overview-cards';
import { useAnalyticsMetadata, useAnalyticsOverview, useAnalyticsDailyStats, useAnalyticsDimensionStats } from './data/analytics';

export default function AnalyticsPage() {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const filter = useAnalyticsFilterStore((state) => state.filter);
  const { data: generalSettings } = useGeneralSettings();

  const currencyCode = generalSettings?.currencyCode || 'USD';

  const { data: metadata } = useAnalyticsMetadata();
  const { data: overview, isLoading: isOverviewLoading } = useAnalyticsOverview(filter);
  const { data: dailyStats, isLoading: isDailyLoading } = useAnalyticsDailyStats(filter);
  const { data: channelStats, isLoading: isChannelLoading } = useAnalyticsDimensionStats(filter, 'channel');
  const { data: modelStats, isLoading: isModelLoading } = useAnalyticsDimensionStats(filter, 'model');
  const { data: apiKeyStats, isLoading: isApiKeyLoading } = useAnalyticsDimensionStats(filter, 'apiKey');
  const { data: userStats, isLoading: isUserLoading } = useAnalyticsDimensionStats(filter, 'user');

  return (
    <div className='flex-1 space-y-6 p-8 pt-6'>
      <Header />
      <Button onClick={() => navigate({ to: '/' })} variant='outline' className='self-start'>
        {t('dashboard.channelSuccessRates.backToDashboard')}
      </Button>
      <AnalyticsFilterBar earliestDate={metadata?.earliestDate} />
      <OverviewCards overview={overview} isLoading={isOverviewLoading} />
      <CombinedTrendChart data={dailyStats || []} isLoading={isDailyLoading} currencyCode={currencyCode} />
      <DimensionPieCharts
        channelStats={channelStats || []}
        modelStats={modelStats || []}
        apiKeyStats={apiKeyStats || []}
        userStats={userStats || []}
        isLoading={isChannelLoading || isModelLoading || isApiKeyLoading || isUserLoading}
        currencyCode={currencyCode}
      />
      <DimensionDetailTable
        channelStats={channelStats || []}
        modelStats={modelStats || []}
        apiKeyStats={apiKeyStats || []}
        userStats={userStats || []}
        isLoading={isChannelLoading || isModelLoading || isApiKeyLoading || isUserLoading}
      />
    </div>
  );
}
