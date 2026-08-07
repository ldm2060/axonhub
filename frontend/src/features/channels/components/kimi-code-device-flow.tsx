import { useEffect } from 'react';
import { AlertCircle, CheckCircle2, Copy, ExternalLink, Loader2, RefreshCw } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { KimiCodeModel, kimiCodeOAuthPoll, kimiCodeOAuthStart } from '../data/kimicode';
import { useDeviceFlow } from '../hooks/use-device-flow';

interface KimiCodeDeviceFlowProps {
  existingCredentials?: string;
  onSuccess: (completion: { credentials: string; models: KimiCodeModel[] }) => void;
  onError?: (error: string) => void;
}

export function KimiCodeDeviceFlow({ existingCredentials, onSuccess, onError }: KimiCodeDeviceFlowProps) {
  const { t } = useTranslation();
  const deviceFlow = useDeviceFlow({
    start: kimiCodeOAuthStart,
    poll: async (input) => {
      const result = await kimiCodeOAuthPoll(input);
      return result.status === 'complete' && result.credentials && result.models
        ? { status: 'complete' as const, completion: { credentials: result.credentials, models: result.models }, message: result.message }
        : result;
    },
    onSuccess,
  });
  useEffect(() => {
    if (deviceFlow.error) onError?.(deviceFlow.error);
  }, [deviceFlow.error, onError]);

  if (existingCredentials?.trim() && !deviceFlow.userCode && !deviceFlow.isComplete && !deviceFlow.error) {
    return (
      <div className='mt-3 rounded-md border border-green-500/50 bg-green-50/10 p-3' data-testid='kimi-code-connected'>
        <div className='flex items-center gap-2 text-green-600'>
          <CheckCircle2 className='h-5 w-5' />
          <span className='font-medium'>{t('channels.dialogs.kimi_code.messages.alreadyConnected')}</span>
        </div>
        <Button
          type='button'
          variant='outline'
          size='sm'
          className='mt-2'
          onClick={() => {
            deviceFlow.reset();
            void deviceFlow.start();
          }}
        >
          <RefreshCw className='mr-2 h-4 w-4' />
          {t('channels.dialogs.kimi_code.buttons.reauthenticate')}
        </Button>
      </div>
    );
  }
  if (deviceFlow.isComplete)
    return (
      <div className='mt-3 rounded-md border border-green-500 p-3' data-testid='kimi-code-auth-complete'>
        <div className='flex items-center gap-2 text-green-600'>
          <CheckCircle2 className='h-5 w-5' />
          <span className='font-medium'>{t('channels.dialogs.kimi_code.messages.authSuccess')}</span>
        </div>
      </div>
    );
  if (deviceFlow.error)
    return (
      <div className='border-destructive mt-3 rounded-md border p-3'>
        <div className='text-destructive flex items-center gap-2'>
          <AlertCircle className='h-5 w-5' />
          <span className='font-medium'>{t('common.error')}</span>
        </div>
        <p className='text-muted-foreground mt-1 text-xs'>{deviceFlow.error}</p>
        <Button type='button' variant='outline' size='sm' className='mt-2' onClick={deviceFlow.reset}>
          <RefreshCw className='mr-2 h-4 w-4' />
          {t('common.buttons.retry')}
        </Button>
      </div>
    );
  if (deviceFlow.userCode)
    return (
      <div className='mt-3 rounded-md border p-3' data-testid='kimi-code-device-flow'>
        <div className='flex items-center gap-2'>
          {deviceFlow.isPolling && <Loader2 className='h-5 w-5 animate-spin' />}
          <span className='font-medium'>{t('channels.dialogs.kimi_code.messages.waitingForAuth')}</span>
        </div>
        <Label className='mt-3 block text-sm'>{t('channels.dialogs.kimi_code.labels.userCode')}</Label>
        <div className='mt-2 flex gap-2'>
          <code className='bg-muted flex-1 rounded-md p-3 text-center text-xl font-bold tracking-wider'>{deviceFlow.userCode}</code>
          <Button
            type='button'
            variant='outline'
            size='icon'
            onClick={() => {
              void navigator.clipboard.writeText(deviceFlow.userCode || '');
              toast.success(t('channels.messages.credentialsCopied'));
            }}
          >
            <Copy className='h-4 w-4' />
          </Button>
        </div>
        <Button
          type='button'
          className='mt-3 w-full'
          variant='secondary'
          onClick={() => window.open(deviceFlow.verificationUri || '', '_blank', 'noopener,noreferrer')}
        >
          <ExternalLink className='mr-2 h-4 w-4' />
          {t('channels.dialogs.kimi_code.buttons.openKimi')}
        </Button>
        <p className='text-muted-foreground mt-2 text-center text-xs'>{deviceFlow.verificationUri}</p>
      </div>
    );
  return (
    <div className='mt-3 rounded-md border p-3'>
      <Button
        type='button'
        variant='secondary'
        onClick={() => void deviceFlow.start()}
        disabled={deviceFlow.isPolling}
        data-testid='kimi-code-start-oauth'
      >
        {t('channels.dialogs.kimi_code.buttons.startOAuth')}
      </Button>
    </div>
  );
}
