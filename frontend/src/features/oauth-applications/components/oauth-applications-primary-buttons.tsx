'use client';

import { Plus } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { useOAuthApplicationsContext } from '../context/oauth-applications-context';

export function OAuthApplicationsPrimaryButtons() {
  const { t } = useTranslation();
  const { setIsCreateDialogOpen } = useOAuthApplicationsContext();

  return (
    <div className='flex flex-wrap items-center gap-2'>
      <Button onClick={() => setIsCreateDialogOpen(true)}>
        <Plus className='mr-2 h-4 w-4' />
        {t('oauthApplications.buttons.create')}
      </Button>
    </div>
  );
}
