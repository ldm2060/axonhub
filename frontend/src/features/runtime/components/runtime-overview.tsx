'use client';

import { Activity, ArrowDown, ArrowUp, Cpu, HardDrive, MemoryStick, Network, RefreshCw, Server } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Area, AreaChart, CartesianGrid, Legend, ResponsiveContainer, Tooltip, XAxis, YAxis } from 'recharts';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Progress } from '@/components/ui/progress';
import { Skeleton } from '@/components/ui/skeleton';
import { useSystemRuntimeOverview } from '../data/runtime';

const CHART_COLORS = {
  systemCpu: 'var(--chart-1)',
  processCpu: 'var(--chart-4)',
  memory: 'var(--chart-2)',
  receive: 'var(--chart-2)',
  transmit: 'var(--chart-3)',
  rss: 'var(--chart-1)',
  heap: 'var(--chart-3)',
};

function formatBytes(value: number, fractionDigits = 1) {
  if (!Number.isFinite(value) || value <= 0) return '0 B';
  const units = ['B', 'KB', 'MB', 'GB', 'TB'];
  const index = Math.min(Math.floor(Math.log(value) / Math.log(1024)), units.length - 1);
  return `${(value / 1024 ** index).toFixed(index === 0 ? 0 : fractionDigits)} ${units[index]}`;
}
const formatRate = (value: number) => `${formatBytes(value)}/s`;
const formatPercent = (value: number) => `${value.toFixed(1)}%`;
function formatDuration(seconds: number) {
  const totalSeconds = Math.max(0, Math.floor(seconds));
  const days = Math.floor(totalSeconds / 86_400);
  const hours = Math.floor((totalSeconds % 86_400) / 3_600);
  const minutes = Math.floor((totalSeconds % 3_600) / 60);
  if (days > 0) return `${days}d ${hours}h ${minutes}m`;
  if (hours > 0) return `${hours}h ${minutes}m`;
  return `${minutes}m`;
}

function MetricCard({
  title,
  value,
  description,
  progress,
  icon: Icon,
}: {
  title: string;
  value: string;
  description: string;
  progress?: number;
  icon: typeof Cpu;
}) {
  return (
    <Card>
      <CardContent className='p-5'>
        <div className='flex items-start justify-between gap-4'>
          <div className='min-w-0 space-y-1'>
            <p className='text-muted-foreground text-sm'>{title}</p>
            <p className='truncate text-2xl font-semibold tracking-tight'>{value}</p>
            <p className='text-muted-foreground text-xs'>{description}</p>
          </div>
          <div className='bg-primary/10 text-primary rounded-xl p-2.5'>
            <Icon className='h-5 w-5' />
          </div>
        </div>
        {progress !== undefined && <Progress value={Math.min(100, Math.max(0, progress))} className='mt-4 h-1.5' />}
      </CardContent>
    </Card>
  );
}

const tooltipStyle = {
  backgroundColor: 'var(--background)',
  borderColor: 'var(--border)',
  borderRadius: 'var(--radius)',
  fontSize: '12px',
};

const tooltipItemStyle = { padding: '2px 0' };

export function RuntimeOverview() {
  const { t, i18n } = useTranslation();
  const { data, isLoading, isError, isFetching, refetch } = useSystemRuntimeOverview();
  if (isLoading)
    return (
      <div className='space-y-6'>
        <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-4'>
          {[1, 2, 3, 4].map((item) => (
            <Skeleton key={item} className='h-36 rounded-xl' />
          ))}
        </div>
        <Skeleton className='h-80 rounded-xl' />
        <Skeleton className='h-80 rounded-xl' />
      </div>
    );
  if (isError || !data)
    return (
      <Alert variant='destructive'>
        <Activity className='h-4 w-4' />
        <AlertTitle>{t('runtime.error.title')}</AlertTitle>
        <AlertDescription className='flex flex-wrap items-center justify-between gap-3'>
          <span>{t('runtime.error.description')}</span>
          <Button variant='outline' size='sm' onClick={() => refetch()}>
            {t('runtime.refresh')}
          </Button>
        </AlertDescription>
      </Alert>
    );

  const locale = i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US';
  const current = data.current;
  const stats = data.stats;
  const timeFormatter = new Intl.DateTimeFormat(locale, { hour: '2-digit', minute: '2-digit' });
  const chartData = data.history.map((sample) => ({
    time: timeFormatter.format(new Date(sample.timestamp)),
    systemCpu: sample.systemCpuPercent,
    processCpu: sample.processCpuPercent,
    memory: sample.memoryUsedPercent,
    receive: sample.networkReceiveBytesPerSecond,
    transmit: sample.networkTransmitBytesPerSecond,
    rss: sample.processRssBytes,
    heap: sample.processHeapAllocBytes,
  }));
  const platform = [data.host.platform, data.host.platformVersion].filter(Boolean).join(' ');
  const collectedAt = new Intl.DateTimeFormat(locale, {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
  }).format(new Date(data.collectedAt));

  return (
    <div className='space-y-6'>
      <div className='flex flex-wrap items-center justify-end gap-3'>
        <Badge variant='outline' className='gap-1.5'>
          <span className='h-2 w-2 rounded-full bg-emerald-500' />
          {t('runtime.live')}
        </Badge>
        <span className='text-muted-foreground hidden text-xs sm:inline'>{t('runtime.updatedAt', { value: collectedAt })}</span>
        <Button variant='outline' size='sm' onClick={() => refetch()} disabled={isFetching}>
          <RefreshCw className={`mr-2 h-4 w-4 ${isFetching ? 'animate-spin' : ''}`} />
          {t('runtime.refresh')}
        </Button>
      </div>

      <div className='grid gap-4 sm:grid-cols-2 xl:grid-cols-4'>
        <MetricCard
          title={t('runtime.metrics.cpu')}
          value={formatPercent(current.systemCpuPercent)}
          description={t('runtime.metrics.cpuDetail', { process: formatPercent(current.processCpuPercent), cores: data.host.logicalCpus })}
          progress={current.systemCpuPercent}
          icon={Cpu}
        />
        <MetricCard
          title={t('runtime.metrics.memory')}
          value={formatPercent(current.memoryUsedPercent)}
          description={t('runtime.metrics.memoryDetail', {
            used: formatBytes(current.memoryUsedBytes),
            total: formatBytes(current.memoryTotalBytes),
          })}
          progress={current.memoryUsedPercent}
          icon={MemoryStick}
        />
        <MetricCard
          title={t('runtime.metrics.network')}
          value={formatRate(current.networkReceiveBytesPerSecond)}
          description={t('runtime.metrics.networkDetail', {
            receive: formatRate(current.networkReceiveBytesPerSecond),
            transmit: formatRate(current.networkTransmitBytesPerSecond),
          })}
          icon={Network}
        />
        <MetricCard
          title={t('runtime.metrics.disk')}
          value={formatPercent(current.diskUsedPercent)}
          description={t('runtime.metrics.diskDetail', {
            used: formatBytes(current.diskUsedBytes),
            total: formatBytes(current.diskTotalBytes),
          })}
          progress={current.diskUsedPercent}
          icon={HardDrive}
        />
      </div>

      <div className='grid gap-6 xl:grid-cols-2'>
        <Card>
          <CardHeader>
            <CardTitle>{t('runtime.charts.resources')}</CardTitle>
            <CardDescription>{t('runtime.charts.lastThreeHours')}</CardDescription>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width='100%' height={290}>
              <AreaChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id='runtime-cpu-fill' x1='0' y1='0' x2='0' y2='1'>
                    <stop offset='5%' stopColor={CHART_COLORS.systemCpu} stopOpacity={0.3} />
                    <stop offset='95%' stopColor={CHART_COLORS.systemCpu} stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id='runtime-memory-fill' x1='0' y1='0' x2='0' y2='1'>
                    <stop offset='5%' stopColor={CHART_COLORS.memory} stopOpacity={0.2} />
                    <stop offset='95%' stopColor={CHART_COLORS.memory} stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id='runtime-process-cpu-fill' x1='0' y1='0' x2='0' y2='1'>
                    <stop offset='5%' stopColor={CHART_COLORS.processCpu} stopOpacity={0.15} />
                    <stop offset='95%' stopColor={CHART_COLORS.processCpu} stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' vertical={false} />
                <XAxis dataKey='time' stroke='var(--muted-foreground)' fontSize={12} tickLine axisLine minTickGap={28} />
                <YAxis
                  domain={[0, 100]}
                  tickFormatter={(value) => `${value}%`}
                  stroke='var(--muted-foreground)'
                  fontSize={12}
                  tickLine
                  axisLine
                  width={40}
                  tickMargin={8}
                />
                <Tooltip
                  isAnimationActive={false}
                  formatter={(value: number | string, name: string) => [formatPercent(Number(value)), name]}
                  contentStyle={tooltipStyle}
                  itemStyle={tooltipItemStyle}
                />
                <Legend verticalAlign='top' height={36} />
                <Area
                  type='monotone'
                  dataKey='systemCpu'
                  name={t('runtime.series.systemCpu')}
                  stroke={CHART_COLORS.systemCpu}
                  strokeWidth={2}
                  fill='url(#runtime-cpu-fill)'
                  fillOpacity={1}
                  dot={{ r: 2, strokeWidth: 0, fill: CHART_COLORS.systemCpu }}
                  activeDot={{ r: 4, strokeWidth: 2, stroke: 'var(--background)' }}
                />
                <Area
                  type='monotone'
                  dataKey='memory'
                  name={t('runtime.series.memory')}
                  stroke={CHART_COLORS.memory}
                  strokeWidth={2}
                  fill='url(#runtime-memory-fill)'
                  fillOpacity={1}
                  dot={{ r: 2, strokeWidth: 0, fill: CHART_COLORS.memory }}
                  activeDot={{ r: 4, strokeWidth: 2, stroke: 'var(--background)' }}
                />
                <Area
                  type='monotone'
                  dataKey='processCpu'
                  name={t('runtime.series.processCpu')}
                  stroke={CHART_COLORS.processCpu}
                  strokeWidth={2}
                  fill='url(#runtime-process-cpu-fill)'
                  fillOpacity={1}
                  dot={{ r: 2, strokeWidth: 0, fill: CHART_COLORS.processCpu }}
                  activeDot={{ r: 4, strokeWidth: 2, stroke: 'var(--background)' }}
                />
              </AreaChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>

        <Card>
          <CardHeader>
            <CardTitle>{t('runtime.charts.network')}</CardTitle>
            <CardDescription>{t('runtime.charts.lastThreeHours')}</CardDescription>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width='100%' height={290}>
              <AreaChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id='runtime-rx-fill' x1='0' y1='0' x2='0' y2='1'>
                    <stop offset='5%' stopColor={CHART_COLORS.receive} stopOpacity={0.3} />
                    <stop offset='95%' stopColor={CHART_COLORS.receive} stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id='runtime-tx-fill' x1='0' y1='0' x2='0' y2='1'>
                    <stop offset='5%' stopColor={CHART_COLORS.transmit} stopOpacity={0.2} />
                    <stop offset='95%' stopColor={CHART_COLORS.transmit} stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' vertical={false} />
                <XAxis dataKey='time' stroke='var(--muted-foreground)' fontSize={12} tickLine axisLine minTickGap={28} />
                <YAxis
                  tickFormatter={(value) => formatBytes(value, 0)}
                  stroke='var(--muted-foreground)'
                  fontSize={12}
                  tickLine
                  axisLine
                  width={58}
                  tickMargin={8}
                />
                <Tooltip
                  isAnimationActive={false}
                  formatter={(value: number | string, name: string) => [formatRate(Number(value)), name]}
                  contentStyle={tooltipStyle}
                  itemStyle={tooltipItemStyle}
                />
                <Legend verticalAlign='top' height={36} />
                <Area
                  type='monotone'
                  dataKey='receive'
                  name={t('runtime.series.receive')}
                  stroke={CHART_COLORS.receive}
                  strokeWidth={2}
                  fill='url(#runtime-rx-fill)'
                  fillOpacity={1}
                  dot={{ r: 2, strokeWidth: 0, fill: CHART_COLORS.receive }}
                  activeDot={{ r: 4, strokeWidth: 2, stroke: 'var(--background)' }}
                />
                <Area
                  type='monotone'
                  dataKey='transmit'
                  name={t('runtime.series.transmit')}
                  stroke={CHART_COLORS.transmit}
                  strokeWidth={2}
                  fill='url(#runtime-tx-fill)'
                  fillOpacity={1}
                  dot={{ r: 2, strokeWidth: 0, fill: CHART_COLORS.transmit }}
                  activeDot={{ r: 4, strokeWidth: 2, stroke: 'var(--background)' }}
                />
              </AreaChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
      </div>

      <div className='grid gap-6 xl:grid-cols-3'>
        <Card className='xl:col-span-2'>
          <CardHeader>
            <CardTitle>{t('runtime.charts.processMemory')}</CardTitle>
            <CardDescription>{t('runtime.charts.processMemoryDescription')}</CardDescription>
          </CardHeader>
          <CardContent>
            <ResponsiveContainer width='100%' height={260}>
              <AreaChart data={chartData} margin={{ top: 10, right: 10, left: 0, bottom: 0 }}>
                <defs>
                  <linearGradient id='runtime-rss-fill' x1='0' y1='0' x2='0' y2='1'>
                    <stop offset='5%' stopColor={CHART_COLORS.rss} stopOpacity={0.3} />
                    <stop offset='95%' stopColor={CHART_COLORS.rss} stopOpacity={0} />
                  </linearGradient>
                  <linearGradient id='runtime-heap-fill' x1='0' y1='0' x2='0' y2='1'>
                    <stop offset='5%' stopColor={CHART_COLORS.heap} stopOpacity={0.2} />
                    <stop offset='95%' stopColor={CHART_COLORS.heap} stopOpacity={0} />
                  </linearGradient>
                </defs>
                <CartesianGrid strokeDasharray='3 3' stroke='var(--border)' vertical={false} />
                <XAxis dataKey='time' stroke='var(--muted-foreground)' fontSize={12} tickLine axisLine minTickGap={28} />
                <YAxis
                  tickFormatter={(value) => formatBytes(value, 0)}
                  stroke='var(--muted-foreground)'
                  fontSize={12}
                  tickLine
                  axisLine
                  width={58}
                  tickMargin={8}
                />
                <Tooltip
                  isAnimationActive={false}
                  formatter={(value: number | string, name: string) => [formatBytes(Number(value)), name]}
                  contentStyle={tooltipStyle}
                  itemStyle={tooltipItemStyle}
                />
                <Legend verticalAlign='top' height={36} />
                <Area
                  type='monotone'
                  dataKey='rss'
                  name={t('runtime.series.rss')}
                  stroke={CHART_COLORS.rss}
                  strokeWidth={2}
                  fill='url(#runtime-rss-fill)'
                  fillOpacity={1}
                  dot={{ r: 2, strokeWidth: 0, fill: CHART_COLORS.rss }}
                  activeDot={{ r: 4, strokeWidth: 2, stroke: 'var(--background)' }}
                />
                <Area
                  type='monotone'
                  dataKey='heap'
                  name={t('runtime.series.heap')}
                  stroke={CHART_COLORS.heap}
                  strokeWidth={2}
                  fill='url(#runtime-heap-fill)'
                  fillOpacity={1}
                  dot={{ r: 2, strokeWidth: 0, fill: CHART_COLORS.heap }}
                  activeDot={{ r: 4, strokeWidth: 2, stroke: 'var(--background)' }}
                />
              </AreaChart>
            </ResponsiveContainer>
          </CardContent>
        </Card>
        <Card>
          <CardHeader>
            <CardTitle className='flex items-center gap-2'>
              <Server className='h-5 w-5' />
              {t('runtime.service.title')}
            </CardTitle>
            <CardDescription>{t('runtime.service.description')}</CardDescription>
          </CardHeader>
          <CardContent className='space-y-3 text-sm'>
            {[
              [t('runtime.service.host'), data.host.hostname],
              [t('runtime.service.platform'), `${platform} (${data.host.architecture})`],
              [t('runtime.service.kernel'), data.host.kernelVersion || '-'],
              [t('runtime.service.version'), data.host.version || '-'],
              [t('runtime.service.goVersion'), data.host.goVersion],
              [t('runtime.service.processId'), String(data.host.processId)],
              [t('runtime.service.uptime'), formatDuration(current.serviceUptimeSeconds)],
              [t('runtime.service.goroutines'), String(current.goroutines)],
              [t('runtime.service.threads'), current.processThreads ? String(current.processThreads) : '-'],
              [
                t('runtime.service.gc'),
                t('runtime.service.gcDetail', { count: current.gcCount, pause: current.gcPauseMilliseconds.toFixed(2) }),
              ],
            ].map(([label, value]) => (
              <div key={label} className='flex items-start justify-between gap-4 border-b pb-2 last:border-0 last:pb-0'>
                <span className='text-muted-foreground'>{label}</span>
                <span className='max-w-[60%] text-right font-medium break-words'>{value}</span>
              </div>
            ))}
          </CardContent>
        </Card>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>{t('runtime.summary.title')}</CardTitle>
          <CardDescription>{t('runtime.summary.description', { count: stats.sampleCount })}</CardDescription>
        </CardHeader>
        <CardContent className='grid gap-4 sm:grid-cols-2 lg:grid-cols-4'>
          <div className='rounded-xl border p-4'>
            <p className='text-muted-foreground text-xs'>{t('runtime.summary.cpu')}</p>
            <p className='mt-1 text-lg font-semibold'>
              {formatPercent(stats.systemCpuAveragePercent)} / {formatPercent(stats.systemCpuMaxPercent)}
            </p>
            <p className='text-muted-foreground text-xs'>{t('runtime.summary.averagePeak')}</p>
          </div>
          <div className='rounded-xl border p-4'>
            <p className='text-muted-foreground text-xs'>{t('runtime.summary.memory')}</p>
            <p className='mt-1 text-lg font-semibold'>
              {formatPercent(stats.memoryUsedAveragePercent)} / {formatPercent(stats.memoryUsedMaxPercent)}
            </p>
            <p className='text-muted-foreground text-xs'>{t('runtime.summary.averagePeak')}</p>
          </div>
          <div className='rounded-xl border p-4'>
            <p className='text-muted-foreground flex items-center gap-1 text-xs'>
              <ArrowDown className='h-3.5 w-3.5' />
              {t('runtime.summary.download')}
            </p>
            <p className='mt-1 text-lg font-semibold'>{formatBytes(stats.networkReceiveTotalBytes)}</p>
            <p className='text-muted-foreground text-xs'>
              {t('runtime.summary.peakRate', { value: formatRate(stats.networkReceivePeakBytesPerSecond) })}
            </p>
          </div>
          <div className='rounded-xl border p-4'>
            <p className='text-muted-foreground flex items-center gap-1 text-xs'>
              <ArrowUp className='h-3.5 w-3.5' />
              {t('runtime.summary.upload')}
            </p>
            <p className='mt-1 text-lg font-semibold'>{formatBytes(stats.networkTransmitTotalBytes)}</p>
            <p className='text-muted-foreground text-xs'>
              {t('runtime.summary.peakRate', { value: formatRate(stats.networkTransmitPeakBytesPerSecond) })}
            </p>
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
