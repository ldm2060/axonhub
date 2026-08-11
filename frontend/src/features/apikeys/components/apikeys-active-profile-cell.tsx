import { Settings } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import LongText from '@/components/long-text';
import { useApiKeysContext } from '../context/apikeys-context';
import type { ApiKey } from '../data/schema';

export function ActiveProfileCell({ apiKey, canWrite }: { apiKey: ApiKey; canWrite: boolean }) {
  const { t } = useTranslation();
  const { openDialog } = useApiKeysContext();
  const activeProfile = apiKey.profiles?.activeProfile?.trim();
  const activeProfileConfig = apiKey.profiles?.profiles?.find((profile) => profile.name === activeProfile);
  const templateName = activeProfileConfig?.templateName?.trim();
  const canOpenProfiles = canWrite && apiKey.type !== 'service_account';

  if (!canOpenProfiles) {
    return activeProfile ? (
      <div className='min-w-0'>
        <LongText className='max-w-36 font-medium'>{activeProfile}</LongText>
        {templateName && (
          <div className='text-muted-foreground max-w-36 truncate text-xs'>
            {t('apikeys.columns.linkedTemplate', { name: templateName })}
          </div>
        )}
      </div>
    ) : (
      <span className='text-muted-foreground text-sm'>{t('apikeys.columns.noActiveProfile')}</span>
    );
  }

  return (
    <Button
      variant='ghost'
      size='sm'
      className='h-8 max-w-44 justify-start gap-1.5 px-2 font-medium'
      onClick={() => openDialog('profiles', apiKey)}
      title={t('apikeys.columns.activeProfileHint')}
    >
      <Settings className='h-3.5 w-3.5 shrink-0' />
      <span className='min-w-0 text-left'>
        <span className={cn('block truncate', !activeProfile && 'text-muted-foreground')}>
          {activeProfile || t('apikeys.columns.noActiveProfile')}
        </span>
        {templateName && (
          <span className='text-muted-foreground block truncate text-xs font-normal'>
            {t('apikeys.columns.linkedTemplate', { name: templateName })}
          </span>
        )}
      </span>
    </Button>
  );
}
