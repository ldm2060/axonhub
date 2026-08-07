'use client';

import { IconTrash } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useApiKeysContext } from '../context/apikeys-context';
import { useDeleteApiKey } from '../data/apikeys';

export function ApiKeysDeleteDialog() {
  const { t } = useTranslation();
  const { isDialogOpen, closeDialog, selectedApiKey, resetRowSelection } = useApiKeysContext();
  const deleteApiKey = useDeleteApiKey();

  if (!selectedApiKey) return null;

  const handleDelete = async () => {
    try {
      await deleteApiKey.mutateAsync(selectedApiKey.id);
      closeDialog('delete');
      resetRowSelection();
    } catch (_error) {
      // Error will be handled by the mutation's error state
    }
  };

  return (
    <ConfirmDialog
      open={isDialogOpen.delete}
      onOpenChange={() => closeDialog('delete')}
      handleConfirm={handleDelete}
      disabled={deleteApiKey.isPending}
      title={
        <span className='text-destructive'>
          <IconTrash className='stroke-destructive mr-1 inline-block' size={18} />
          {t('apikeys.dialogs.delete.title')}
        </span>
      }
      desc={t('apikeys.dialogs.delete.description')}
      confirmText={t('apikeys.actions.delete')}
      cancelBtnText={t('common.buttons.cancel')}
      destructive
    />
  );
}
