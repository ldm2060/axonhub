import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';

export function MonitorErrorFallback({ channelName, onRetry }: { channelName?: string; onRetry: () => void }) {
  const { t } = useTranslation('usage-monitor');
  return (
    <div className='rounded-lg border border-red-200 bg-red-50 p-4'>
      <p className='text-sm font-medium text-red-800'>
        {channelName ? `${channelName}: ` : ''}
        {t('usageMonitor.status.error')}
      </p>
      <Button variant='outline' size='sm' className='mt-2' onClick={onRetry}>
        {t('usageMonitor.refreshNow')}
      </Button>
    </div>
  );
}
