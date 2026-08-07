import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { useModelPerformanceStats, ModelPerformanceStat, type DashboardMode } from '../data/dashboard';
import { PerformanceChart, PerformanceDataPoint } from './performance-chart';

interface ModelPerformanceStatsProps {
  onTotalRequestsChange?: (total: number) => void;
  mode: DashboardMode;
}

export function ModelPerformanceStats({ onTotalRequestsChange, mode }: ModelPerformanceStatsProps) {
  const { t } = useTranslation();
  const { data: performanceStats, isLoading, error } = useModelPerformanceStats(mode);

  const mappedData: PerformanceDataPoint[] | undefined = useMemo(
    () =>
      performanceStats?.map((stat: ModelPerformanceStat) => ({
        id: stat.modelId,
        name: stat.modelId,
        throughput: stat.throughput,
        ttftMs: stat.ttftMs,
        requestCount: stat.requestCount,
        date: stat.date,
      })),
    [performanceStats]
  );

  return (
    <PerformanceChart
      data={mappedData}
      isLoading={isLoading}
      error={error}
      onTotalRequestsChange={onTotalRequestsChange}
      emptyMessage={t('dashboard.charts.noModelData')}
      errorMessage={t('dashboard.charts.errorLoadingModelData')}
      idField='modelId'
    />
  );
}
