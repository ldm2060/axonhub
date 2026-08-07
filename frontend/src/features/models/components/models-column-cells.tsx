import { useCallback, useState } from 'react';
import type { Row } from '@tanstack/react-table';
import { IconLink } from '@tabler/icons-react';
import { useAuthStore } from '@/stores/authStore';
import { usePermissions } from '@/hooks/usePermissions';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Switch } from '@/components/ui/switch';
import { useModels } from '../context/models-context';
import type { Model } from '../data/schema';
import { useDeveloperLabel } from '../hooks/use-developer-label';
import { ModelsStatusDialog } from './models-status-dialog';

export function StatusSwitchCell({ row }: { row: Row<Model> }) {
  const model = row.original;
  const { modelPermissions } = usePermissions();
  const { user: authUser } = useAuthStore((state) => state.auth);
  const [dialogOpen, setDialogOpen] = useState(false);
  const { channelPermissions } = usePermissions();

  const isEnabled = model.status === 'enabled';
  const isArchived = model.status === 'archived';
  const isOwner = model.ownerID === authUser?.id;
  const canToggle = modelPermissions.canWrite || (modelPermissions.canManageOwn && isOwner);

  const handleSwitchClick = useCallback(() => {
    if (canToggle && !isArchived) {
      setDialogOpen(true);
    }
  }, [canToggle, isArchived]);

  if (!channelPermissions.canWrite) {
    return <Badge variant='outline'>{model.status}</Badge>;
  }

  return (
    <>
      <Switch
        checked={isEnabled}
        onCheckedChange={handleSwitchClick}
        disabled={!canToggle || isArchived}
        data-testid='model-status-switch'
      />
      {dialogOpen && <ModelsStatusDialog open={dialogOpen} onOpenChange={setDialogOpen} currentRow={model} />}
    </>
  );
}

export function DeveloperCell({ row }: { row: Row<Model> }) {
  const getDeveloperLabel = useDeveloperLabel();
  return <Badge variant='outline'>{getDeveloperLabel(row.getValue('developer'))}</Badge>;
}

export function AssociationRulesCell({ row }: { row: Row<Model> }) {
  const model = row.original;
  const { setOpen, setCurrentRow } = useModels();
  const { channelPermissions } = usePermissions();

  const handleOpenAssociationDialog = useCallback(() => {
    setCurrentRow(model);
    setOpen('association');
  }, [model, setCurrentRow, setOpen]);

  const associationCount = model.settings?.associations?.length || 0;

  if (!channelPermissions.canWrite) {
    return (
      <div className='flex justify-center'>
        <Badge variant='secondary'>{associationCount}</Badge>
      </div>
    );
  }

  return (
    <Button size='sm' variant='outline' className='h-8 px-3' onClick={handleOpenAssociationDialog}>
      <IconLink className='mr-1 h-3 w-3' />
      {`${associationCount}`}
    </Button>
  );
}
