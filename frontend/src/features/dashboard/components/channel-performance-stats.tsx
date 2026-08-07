import { useTranslation } from 'react-i18next';
import { useChannelPerformanceStats, type DashboardMode } from '../data/dashboard';
import { PerformanceChart } from './performance-chart';

interface ChannelPerformanceStatsProps {
  onTotalRequestsChange?: (total: number) => void;
  mode: DashboardMode;
}

export function ChannelPerformanceStats({ onTotalRequestsChange, mode }: ChannelPerformanceStatsProps) {
  const { t } = useTranslation();
  const { data: performanceStats, isLoading, error } = useChannelPerformanceStats(mode);

  const mappedData = performanceStats?.map((stat) => ({
    date: stat.date,
    id: stat.channelId,
    name: stat.channelName,
    throughput: stat.throughput,
    ttftMs: stat.ttftMs,
    requestCount: stat.requestCount,
  }));

  return (
    <PerformanceChart
      data={mappedData}
      isLoading={isLoading}
      error={error}
      onTotalRequestsChange={onTotalRequestsChange}
      emptyMessage={t('dashboard.charts.noChannelData')}
      errorMessage={t('dashboard.charts.errorLoadingChannelData')}
      idField='channelId'
      nameField='channelName'
    />
  );
}
