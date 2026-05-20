import { IconPlus } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { PermissionGuard } from '@/components/permission-guard';
import { useChannels } from '../context/channels-context';

export function PersonalChannelsButtons() {
  const { t } = useTranslation();
  const { setOpen } = useChannels();

  return (
    <div className='flex gap-2 overflow-x-auto md:overflow-x-visible'>
      <PermissionGuard requiredScope='manage_own_channels'>
        <Button className='shrink-0 space-x-1' onClick={() => setOpen('add')} data-testid='add-channel-button'>
          <span>{t('channels.addChannel')}</span> <IconPlus size={18} />
        </Button>
      </PermissionGuard>
    </div>
  );
}
