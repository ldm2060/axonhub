'use client';

import { Ban, KeyRound, MoreHorizontal, Pencil, Play, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { DropdownMenu, DropdownMenuContent, DropdownMenuItem, DropdownMenuTrigger } from '@/components/ui/dropdown-menu';
import { usePermissions } from '@/hooks/usePermissions';
import { useOAuthApplicationsContext } from '../context/oauth-applications-context';
import { OAuthApplication, useUpdateOAuthApplication } from '../data/oauth-applications';

interface OAuthApplicationActionsProps {
  application: OAuthApplication;
}

export function OAuthApplicationActions({ application }: OAuthApplicationActionsProps) {
  const { t } = useTranslation();
  const { isOwner } = usePermissions();
  const updateApplication = useUpdateOAuthApplication();
  const {
    setEditingApplication,
    setIsEditDialogOpen,
    setDeletingApplication,
    setIsDeleteDialogOpen,
    setRotatingApplication,
    setIsRotateSecretDialogOpen,
  } = useOAuthApplicationsContext();

  if (!isOwner) {
    return null;
  }

  const isEnabled = application.status === 'enabled';

  const handleToggleStatus = () => {
    updateApplication.mutate({
      id: application.id,
      input: { status: isEnabled ? 'disabled' : 'enabled' },
    });
  };

  return (
    <DropdownMenu>
      <DropdownMenuTrigger asChild>
        <Button variant='ghost' className='h-8 w-8 p-0'>
          <span className='sr-only'>{t('common.buttons.openMenu')}</span>
          <MoreHorizontal className='h-4 w-4' />
        </Button>
      </DropdownMenuTrigger>
      <DropdownMenuContent align='end'>
        <DropdownMenuItem
          onClick={() => {
            setEditingApplication(application);
            setIsEditDialogOpen(true);
          }}
        >
          <Pencil className='mr-2 h-4 w-4' />
          {t('common.buttons.edit')}
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => {
            setRotatingApplication(application);
            setIsRotateSecretDialogOpen(true);
          }}
        >
          <KeyRound className='mr-2 h-4 w-4' />
          {t('oauthApplications.buttons.rotateSecret')}
        </DropdownMenuItem>
        <DropdownMenuItem onClick={handleToggleStatus}>
          {isEnabled ? <Ban className='mr-2 h-4 w-4' /> : <Play className='mr-2 h-4 w-4' />}
          {isEnabled ? t('common.buttons.disable') : t('common.buttons.enable')}
        </DropdownMenuItem>
        <DropdownMenuItem
          onClick={() => {
            setDeletingApplication(application);
            setIsDeleteDialogOpen(true);
          }}
          className='text-destructive focus:text-destructive'
        >
          <Trash2 className='mr-2 h-4 w-4' />
          {t('common.buttons.delete')}
        </DropdownMenuItem>
      </DropdownMenuContent>
    </DropdownMenu>
  );
}
