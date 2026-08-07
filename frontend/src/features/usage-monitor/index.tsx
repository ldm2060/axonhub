import { lazy, Suspense } from 'react';
import { Plus } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { MonitorCard } from './components/monitor-card';
import { MonitorErrorBoundary } from './components/monitor-error-boundary';
import { useUsageMonitorContext } from './context/usage-monitor-context';
import UsageMonitorProvider from './context/usage-monitor-provider';
import { useUsageMonitorChannels } from './data/usage-monitor';

const AddChannelDialog = lazy(() => import('./components/add-channel-dialog').then((m) => ({ default: m.AddChannelDialog })));
const EditChannelDialog = lazy(() => import('./components/edit-channel-dialog').then((m) => ({ default: m.EditChannelDialog })));
const DeleteChannelDialog = lazy(() => import('./components/delete-channel-dialog').then((m) => ({ default: m.DeleteChannelDialog })));

function UsageMonitorPrimaryButtons() {
  const { t } = useTranslation();
  const { setOpen } = useUsageMonitorContext();

  return (
    <Button onClick={() => setOpen('add')}>
      <Plus className='mr-2 size-4' />
      {t('usageMonitor.addChannel')}
    </Button>
  );
}

function UsageMonitorContent() {
  const { t } = useTranslation();
  const { setOpen } = useUsageMonitorContext();
  const { data: channels = [], isLoading } = useUsageMonitorChannels();

  if (isLoading) {
    return (
      <div className='grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4'>
        {Array.from({ length: 4 }).map((_, i) => (
          <div key={i} className='bg-muted h-48 animate-pulse rounded-lg border p-4' />
        ))}
      </div>
    );
  }

  if (channels.length === 0) {
    return (
      <div className='flex flex-1 flex-col items-center justify-center gap-4 py-16'>
        <p className='text-muted-foreground text-sm'>{t('usageMonitor.noData')}</p>
        <Button onClick={() => setOpen('add')}>
          <Plus className='mr-2 size-4' />
          {t('usageMonitor.addChannel')}
        </Button>
      </div>
    );
  }

  return (
    <div className='flex-1 overflow-y-auto'>
      <div className='grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-4'>
        {channels.map((channel) => (
          <MonitorErrorBoundary key={channel.id} channelName={channel.name}>
            <MonitorCard channel={channel} />
          </MonitorErrorBoundary>
        ))}
      </div>
    </div>
  );
}

export default function UsageMonitorPage() {
  const { t } = useTranslation();

  return (
    <UsageMonitorProvider>
      <Header fixed>
        <div className='flex w-full flex-1 flex-col gap-2 md:flex-row md:items-center md:justify-between md:gap-0'>
          <div className='min-w-0'>
            <h2 className='text-xl font-bold tracking-tight'>{t('usageMonitor.title')}</h2>
            <p className='text-muted-foreground text-sm'>{t('usageMonitor.description')}</p>
          </div>
          <UsageMonitorPrimaryButtons />
        </div>
      </Header>

      <Main fixed>
        <UsageMonitorContent />
      </Main>

      <Suspense fallback={null}>
        <AddChannelDialog />
        <EditChannelDialog />
        <DeleteChannelDialog />
      </Suspense>
    </UsageMonitorProvider>
  );
}
