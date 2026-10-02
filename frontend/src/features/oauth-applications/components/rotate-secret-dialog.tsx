'use client';

import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Alert, AlertDescription } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useOAuthApplicationsContext } from '../context/oauth-applications-context';
import { useRotateOAuthApplicationSecret } from '../data/oauth-applications';
import { ClientSecretReveal } from './client-secret-reveal';

export function RotateSecretDialog() {
  const { t } = useTranslation();
  const { isRotateSecretDialogOpen, setIsRotateSecretDialogOpen, rotatingApplication, setRotatingApplication } =
    useOAuthApplicationsContext();
  const rotateMutation = useRotateOAuthApplicationSecret();
  const [newSecret, setNewSecret] = useState<string | null>(null);

  if (!rotatingApplication) {
    return null;
  }

  const handleClose = () => {
    setIsRotateSecretDialogOpen(false);
    setRotatingApplication(null);
    setNewSecret(null);
  };

  const handleRotate = async () => {
    try {
      const result = await rotateMutation.mutateAsync(rotatingApplication.id);
      setNewSecret(result.clientSecret);
    } catch {
      // Error is handled by the mutation
    }
  };

  return (
    <Dialog open={isRotateSecretDialogOpen} onOpenChange={handleClose}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader className='text-left'>
          <DialogTitle>{t('oauthApplications.dialogs.rotateSecret.title')}</DialogTitle>
          <DialogDescription>
            {t('oauthApplications.dialogs.rotateSecret.description', { name: rotatingApplication.name })}
          </DialogDescription>
        </DialogHeader>

        {newSecret ? (
          <div className='space-y-4'>
            <ClientSecretReveal clientSecret={newSecret} />
            <DialogFooter>
              <Button onClick={handleClose}>{t('common.buttons.close')}</Button>
            </DialogFooter>
          </div>
        ) : (
          <div className='space-y-4'>
            <Alert className='border-orange-200 bg-orange-50 dark:border-orange-800 dark:bg-orange-950'>
              <AlertDescription className='text-orange-800 dark:text-orange-200'>
                {t('oauthApplications.dialogs.rotateSecret.warning')}
              </AlertDescription>
            </Alert>
            <DialogFooter>
              <Button variant='outline' onClick={handleClose} disabled={rotateMutation.isPending}>
                {t('common.buttons.cancel')}
              </Button>
              <Button onClick={handleRotate} disabled={rotateMutation.isPending}>
                {rotateMutation.isPending ? t('common.buttons.processing') : t('oauthApplications.buttons.rotateSecret')}
              </Button>
            </DialogFooter>
          </div>
        )}
      </DialogContent>
    </Dialog>
  );
}
