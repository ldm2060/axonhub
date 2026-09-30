'use client';

import { CheckIcon, Copy } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { useCopyToClipboard } from '@/hooks/use-copy-to-clipboard';

interface ClientSecretRevealProps {
  clientSecret: string;
}

export function ClientSecretReveal({ clientSecret }: ClientSecretRevealProps) {
  const { t } = useTranslation();
  const { isCopied, handleCopy } = useCopyToClipboard({ text: clientSecret });

  return (
    <div className='space-y-4'>
      <Alert className='border-orange-200 bg-orange-50 dark:border-orange-800 dark:bg-orange-950'>
        <AlertDescription className='text-orange-800 dark:text-orange-200'>
          {t('oauthApplications.secret.warning')}
        </AlertDescription>
      </Alert>

      <div>
        <label className='text-sm font-medium'>{t('oauthApplications.secret.label')}</label>
        <div className='mt-1 flex items-center space-x-2'>
          <code className='bg-muted flex-1 rounded-md p-3 font-mono text-sm break-all'>{clientSecret}</code>
          <Button variant='outline' size='sm' onClick={handleCopy} className='flex-shrink-0'>
            {isCopied ? <CheckIcon className='h-4 w-4' /> : <Copy className='h-4 w-4' />}
          </Button>
        </div>
      </div>
    </div>
  );
}
