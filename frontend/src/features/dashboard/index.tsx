import { useState, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Link } from '@tanstack/react-router';
import { BarChart3, Brain, Key, Users, Zap, ChevronRight, TrendingUp } from 'lucide-react';
import { Card, CardAction, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Skeleton } from '@/components/ui/skeleton';
import { Header } from '@/components/layout/header';
import { formatNumber } from '@/utils/format-number';
import { TimePeriodSelector, type TimePeriod } from '@/components/time-period-selector';
import { ChannelSuccessRate } from './components/channel-success-rate';
import { DailyRequestStats } from './components/daily-requests-stats';
import { RequestsByChannelChart } from './components/requests-by-channel-chart';
import { RequestsByModelChart } from './components/requests-by-model-chart';
import { RequestsByAPIKeyChart } from './components/requests-by-api-key-chart';
import { TokensByAPIKeyChart } from './components/tokens-by-api-key-chart';
import { TokensByChannelChart } from './components/tokens-by-channel-chart';
import { TokensByModelChart } from './components/tokens-by-model-chart';
import { SuccessRateCard } from './components/success-rate-card';
import { TodayRequestsCard } from './components/today-requests-card';
import { TokenStatsCard } from './components/token-stats-card';
import { TotalRequestsCard } from './components/total-requests-card';
import { FastestChannelsCard } from './components/fastest-channels-card';
import { FastestModelsCard } from './components/fastest-models-card';
import { ModelPerformanceStats } from './components/model-performance-stats';
import { ChannelPerformanceStats } from './components/channel-performance-stats';
import { CollapsibleSection } from './components/collapsible-section';
import { UserUsageBarChart } from './components/user-usage-bar-chart';
import { WeeklyUsageLineChart } from './components/weekly-usage-line-chart';
import { useDashboardStats, type DashboardMode } from './data/dashboard';
import { useRoutePermissions } from '@/hooks/useRoutePermissions';

interface DashboardPageProps {
  mode: DashboardMode;
}

export default function DashboardPage({ mode }: DashboardPageProps) {
  const { t } = useTranslation();
  const { isLoading, error } = useDashboardStats(mode);
  const [modelTotalRequests, setModelTotalRequests] = useState(0);
  const [channelTotalRequests, setChannelTotalRequests] = useState(0);

  const [channelTimePeriod, setChannelTimePeriod] = useState<TimePeriod>('allTime');
  const [channelTokensTimePeriod, setChannelTokensTimePeriod] = useState<TimePeriod>('allTime');
  const [modelTimePeriod, setModelTimePeriod] = useState<TimePeriod>('allTime');
  const [modelTokensTimePeriod, setModelTokensTimePeriod] = useState<TimePeriod>('allTime');
  const [apiKeyTimePeriod, setApiKeyTimePeriod] = useState<TimePeriod>('allTime');
  const [apiKeyTokensTimePeriod, setApiKeyTokensTimePeriod] = useState<TimePeriod>('allTime');
  const [userTimePeriod, setUserTimePeriod] = useState<TimePeriod>('month');

  const modelPerformanceDescription = useMemo(() => {
    return t('dashboard.charts.performanceDescription', { count: formatNumber(modelTotalRequests) });
  }, [t, modelTotalRequests]);

  const channelPerformanceDescription = useMemo(() => {
    return t('dashboard.charts.performanceDescription', { count: formatNumber(channelTotalRequests) });
  }, [t, channelTotalRequests]);

  if (isLoading) {
    return (
      <div className='w-full space-y-4 p-8 pt-6'>
        <div className='flex items-center justify-between space-y-2'>
          <Skeleton className='h-8 w-[200px]' />
        </div>
        <div className='space-y-4'>
          <div className='grid gap-4 md:grid-cols-1 lg:grid-cols-4'>
            <Skeleton className='h-[180px]' />
            <Skeleton className='h-[180px]' />
            <Skeleton className='h-[180px]' />
            <Skeleton className='h-[180px]' />
          </div>
          <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-7'>
            <Skeleton className='col-span-1 h-[300px] lg:col-span-4' />
            <Skeleton className='col-span-1 h-[300px] lg:col-span-3' />
          </div>
        </div>
      </div>
    );
  }

  if (error) {
    return (
      <div className='w-full space-y-4 p-8 pt-6'>
        <div className='text-red-500'>
          {t('common.loadError')} {error.message}
        </div>
      </div>
    );
  }

  return (
    <div className='w-full space-y-6 p-8 pt-6'>
      <Header />

      <DashboardContent
        mode={mode}
        modelPerformanceDescription={modelPerformanceDescription}
        channelPerformanceDescription={channelPerformanceDescription}
        channelTimePeriod={channelTimePeriod}
        setChannelTimePeriod={setChannelTimePeriod}
        channelTokensTimePeriod={channelTokensTimePeriod}
        setChannelTokensTimePeriod={setChannelTokensTimePeriod}
        modelTimePeriod={modelTimePeriod}
        setModelTimePeriod={setModelTimePeriod}
        modelTokensTimePeriod={modelTokensTimePeriod}
        setModelTokensTimePeriod={setModelTokensTimePeriod}
        apiKeyTimePeriod={apiKeyTimePeriod}
        setApiKeyTimePeriod={setApiKeyTimePeriod}
        apiKeyTokensTimePeriod={apiKeyTokensTimePeriod}
        setApiKeyTokensTimePeriod={setApiKeyTokensTimePeriod}
        userTimePeriod={userTimePeriod}
        setUserTimePeriod={setUserTimePeriod}
        modelTotalRequests={modelTotalRequests}
        setModelTotalRequests={setModelTotalRequests}
        channelTotalRequests={channelTotalRequests}
        setChannelTotalRequests={setChannelTotalRequests}
      />

    </div>
  );
}

interface DashboardContentProps {
  mode: DashboardMode;
  modelPerformanceDescription: string;
  channelPerformanceDescription: string;
  channelTimePeriod: TimePeriod;
  setChannelTimePeriod: (v: TimePeriod) => void;
  channelTokensTimePeriod: TimePeriod;
  setChannelTokensTimePeriod: (v: TimePeriod) => void;
  modelTimePeriod: TimePeriod;
  setModelTimePeriod: (v: TimePeriod) => void;
  modelTokensTimePeriod: TimePeriod;
  setModelTokensTimePeriod: (v: TimePeriod) => void;
  apiKeyTimePeriod: TimePeriod;
  setApiKeyTimePeriod: (v: TimePeriod) => void;
  apiKeyTokensTimePeriod: TimePeriod;
  setApiKeyTokensTimePeriod: (v: TimePeriod) => void;
  userTimePeriod: TimePeriod;
  setUserTimePeriod: (v: TimePeriod) => void;
  modelTotalRequests: number;
  setModelTotalRequests: (v: number) => void;
  channelTotalRequests: number;
  setChannelTotalRequests: (v: number) => void;
}

function DashboardContent({
  mode,
  modelPerformanceDescription,
  channelPerformanceDescription,
  channelTimePeriod,
  setChannelTimePeriod,
  channelTokensTimePeriod,
  setChannelTokensTimePeriod,
  modelTimePeriod,
  setModelTimePeriod,
  modelTokensTimePeriod,
  setModelTokensTimePeriod,
  apiKeyTimePeriod,
  setApiKeyTimePeriod,
  apiKeyTokensTimePeriod,
  setApiKeyTokensTimePeriod,
  userTimePeriod,
  setUserTimePeriod,
  modelTotalRequests: _modelTotalRequests,
  setModelTotalRequests,
  channelTotalRequests: _channelTotalRequests,
  setChannelTotalRequests,
}: DashboardContentProps) {
  const { t } = useTranslation();
  const { isProjectOwner, checkRouteAccess } = useRoutePermissions();
  const canAccessAnalytics = mode === 'project' && checkRouteAccess('/analytics').hasAccess;

  return (
    <div className='space-y-6 pt-2'>
      {/* Overview section - always shown */}
      <section className='space-y-4'>
        <div className='grid gap-6 md:grid-cols-2 lg:grid-cols-4'>
          <TotalRequestsCard mode={mode} />
          <SuccessRateCard mode={mode} />
          <TokenStatsCard mode={mode} />
          <TodayRequestsCard mode={mode} />
        </div>
        <div className='grid gap-4 md:grid-cols-2 lg:grid-cols-7'>
          <Card className='hover-card col-span-1 lg:col-span-4'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.dailyRequestOverview')}</CardTitle>
            </CardHeader>
            <CardContent className='pl-2'>
              <DailyRequestStats mode={mode} />
            </CardContent>
          </Card>
          <Card className='hover-card col-span-1 lg:col-span-3'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.channelSuccessRate')}</CardTitle>
              <CardDescription>{t('dashboard.charts.channelSuccessRateDescription')}</CardDescription>
              <CardAction>
                {mode === 'project' && (
                  <Link to='/admin/dashboard/channel-success-rates' className='text-sm text-primary hover:underline'>
                    {t('dashboard.viewAll')}
                  </Link>
                )}
              </CardAction>
            </CardHeader>
            <CardContent>
              <ChannelSuccessRate mode={mode} />
            </CardContent>
          </Card>
        </div>
      </section>

      {/* 使用详情分析 - 导航卡片 */}
      {canAccessAnalytics && (
      <Link
        to='/analytics'
        className='flex w-full items-center justify-between rounded-lg border bg-card p-4 text-left transition-colors hover:bg-accent/50'
      >
        <div className='flex items-center gap-3'>
          <div className='flex h-8 w-8 items-center justify-center rounded-md bg-primary/10'>
            <TrendingUp className='h-4 w-4 text-primary' />
          </div>
          <div>
            <span className='text-lg font-semibold'>{t('dashboard.sections.analytics')}</span>
            <p className='text-sm text-muted-foreground'>{t('dashboard.sections.analyticsDescription')}</p>
          </div>
        </div>
        <ChevronRight className='h-5 w-5 text-muted-foreground' />
      </Link>
      )}

      {/* 渠道分析 - 可折叠 */}
      <CollapsibleSection
        title={t('dashboard.sections.channels')}
        icon={<BarChart3 className='h-4 w-4 text-primary' />}
        storageKey='channels'
      >
        <div className='grid gap-4 md:grid-cols-2'>
          <Card className='hover-card'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.requestsCostByChannel')}</CardTitle>
              <CardDescription>{t('dashboard.charts.requestsCostByChannelDescription')}</CardDescription>
              <CardAction>
                <TimePeriodSelector value={channelTimePeriod} onChange={setChannelTimePeriod} />
              </CardAction>
            </CardHeader>
            <CardContent>
              <RequestsByChannelChart timePeriod={channelTimePeriod} mode={mode} />
            </CardContent>
          </Card>
          <Card className='hover-card'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.tokensByChannel')}</CardTitle>
              <CardDescription>{t('dashboard.charts.tokensByChannelDescription')}</CardDescription>
              <CardAction>
                <TimePeriodSelector value={channelTokensTimePeriod} onChange={setChannelTokensTimePeriod} />
              </CardAction>
            </CardHeader>
            <CardContent>
              <TokensByChannelChart timePeriod={channelTokensTimePeriod} mode={mode} />
            </CardContent>
          </Card>
        </div>
      </CollapsibleSection>

      {/* Model Analytics - collapsible */}
      <CollapsibleSection
        title={t('dashboard.sections.models')}
        icon={<Brain className='h-4 w-4 text-primary' />}
        storageKey='models'
      >
        <div className='grid gap-4 md:grid-cols-2'>
          <Card className='hover-card'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.requestsCostByModel')}</CardTitle>
              <CardDescription>{t('dashboard.charts.requestsCostByModelDescription')}</CardDescription>
              <CardAction>
                <TimePeriodSelector value={modelTimePeriod} onChange={setModelTimePeriod} />
              </CardAction>
            </CardHeader>
            <CardContent>
              <RequestsByModelChart timePeriod={modelTimePeriod} mode={mode} />
            </CardContent>
          </Card>
          <Card className='hover-card'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.tokensByModel')}</CardTitle>
              <CardDescription>{t('dashboard.charts.tokensByModelDescription')}</CardDescription>
              <CardAction>
                <TimePeriodSelector value={modelTokensTimePeriod} onChange={setModelTokensTimePeriod} />
              </CardAction>
            </CardHeader>
            <CardContent>
              <TokensByModelChart timePeriod={modelTokensTimePeriod} mode={mode} />
            </CardContent>
          </Card>
        </div>
      </CollapsibleSection>

      {/* API Key Analytics - collapsible */}
      <CollapsibleSection
        title={t('dashboard.sections.apiKeys')}
        icon={<Key className='h-4 w-4 text-primary' />}
        storageKey='apiKeys'
      >
        <div className='grid gap-4 md:grid-cols-2'>
          <Card className='hover-card'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.requestsCostByAPIKey')}</CardTitle>
              <CardDescription>{t('dashboard.charts.requestsCostByAPIKeyDescription')}</CardDescription>
              <CardAction>
                <TimePeriodSelector value={apiKeyTimePeriod} onChange={setApiKeyTimePeriod} />
              </CardAction>
            </CardHeader>
            <CardContent>
              <RequestsByAPIKeyChart timePeriod={apiKeyTimePeriod} mode={mode} />
            </CardContent>
          </Card>
          <Card className='hover-card'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.tokensByAPIKey')}</CardTitle>
              <CardDescription>{t('dashboard.charts.tokensByAPIKeyDescription')}</CardDescription>
              <CardAction>
                <TimePeriodSelector value={apiKeyTokensTimePeriod} onChange={setApiKeyTokensTimePeriod} />
              </CardAction>
            </CardHeader>
            <CardContent>
              <TokensByAPIKeyChart timePeriod={apiKeyTokensTimePeriod} mode={mode} />
            </CardContent>
          </Card>
        </div>
      </CollapsibleSection>

      {/* 用户分析 - 可折叠 */}
      {(mode === 'personal' || isProjectOwner) && (
        <CollapsibleSection
          title={t('dashboard.sections.users')}
          icon={<Users className='h-4 w-4 text-primary' />}
          storageKey='users'
        >
          <div className='flex justify-end'>
            <TimePeriodSelector value={userTimePeriod} onChange={setUserTimePeriod} />
          </div>
          <div className='grid gap-4 md:grid-cols-2'>
            <UserUsageBarChart timePeriod={userTimePeriod} metric='requests' mode={mode} />
            <WeeklyUsageLineChart mode={mode} />
          </div>
        </CollapsibleSection>
      )}

      {/* Performance Analytics - collapsible */}
      <CollapsibleSection
        title={t('dashboard.sections.performance')}
        icon={<Zap className='h-4 w-4 text-primary' />}
        storageKey='performance'
      >
        <div className='grid gap-4 md:grid-cols-1 lg:grid-cols-7'>
          <Card className='hover-card col-span-1 lg:col-span-4'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.modelPerformance')}</CardTitle>
              <CardDescription>{modelPerformanceDescription}</CardDescription>
            </CardHeader>
            <CardContent>
              <ModelPerformanceStats onTotalRequestsChange={setModelTotalRequests} mode={mode} />
            </CardContent>
          </Card>
          <div className='col-span-1 lg:col-span-3'>
            <FastestModelsCard mode={mode} />
          </div>
        </div>
        <div className='grid gap-4 md:grid-cols-1 lg:grid-cols-7'>
          <Card className='hover-card col-span-1 lg:col-span-4'>
            <CardHeader>
              <CardTitle>{t('dashboard.charts.channelPerformance')}</CardTitle>
              <CardDescription>{channelPerformanceDescription}</CardDescription>
            </CardHeader>
            <CardContent>
              <ChannelPerformanceStats onTotalRequestsChange={setChannelTotalRequests} mode={mode} />
            </CardContent>
          </Card>
          <div className='col-span-1 lg:col-span-3'>
            <FastestChannelsCard mode={mode} />
          </div>
        </div>
      </CollapsibleSection>
    </div>
  );
}
