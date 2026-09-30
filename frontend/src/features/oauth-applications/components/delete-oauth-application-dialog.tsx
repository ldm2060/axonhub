'use client';

import { useTranslation } from 'react-i18next';
import { ConfirmDialog } from '@/components/confirm-dialog';
import { useOAuthApplicationsContext } from '../context/oauth-applications-context';
import { useDeleteOAuthApplication } from '../data/oauth-applications';

export function DeleteOAuthApplicationDialog() {
  const { t } = useTranslation();
  const { isDeleteDialogOpen, setIsDeleteDialogOpen, deletingApplication, setDeletingApplication } = useOAuthApplicationsContext();
  const deleteMutation = useDeleteOAuthApplication();

  if (!deletingApplication) {
    return null;
  }

  const handleDelete = async () => {
    try {
      await deleteMutation.mutateAsync(deletingApplication.id);
      setIsDeleteDialogOpen(false);
      setDeletingApplication(null);
    } catch {
      // Error is handled by the mutation
    }
  };

  return (
    <ConfirmDialog
      open={isDeleteDialogOpen}
      onOpenChange={() => {
        setIsDeleteDialogOpen(false);
        setDeletingApplication(null);
      }}
      handleConfirm={handleDelete}
      destructive
      disabled={deleteMutation.isPending}
      title={t('oauthApplications.dialogs.delete.title')}
      desc={t('oauthApplications.dialogs.delete.description', { name: deletingApplication.name })}
      confirmText={deleteMutation.isPending ? t('common.buttons.processing') : t('common.buttons.delete')}
      cancelBtnText={t('common.buttons.cancel')}
    />
  );
}
