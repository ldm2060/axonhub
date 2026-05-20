import { IconPlus } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { PermissionGuard } from '@/components/permission-guard';
import { useModels } from '../context/models-context';

export function ModelsPersonalButtons() {
  const { t } = useTranslation();
  const { setOpen } = useModels();

  return (
    <div className='flex gap-2 overflow-x-auto md:overflow-x-visible'>
      <PermissionGuard requiredScope='write_channels'>
        <Button className='shrink-0' onClick={() => setOpen('create')}>
          <IconPlus className='mr-2 h-4 w-4' />
          {t('models.actions.create')}
        </Button>
      </PermissionGuard>
    </div>
  );
}
