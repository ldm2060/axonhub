import { useState } from 'react';
import { IconArchive, IconPin, IconRotate } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
} from '@/components/ui/alert-dialog';
import { Button } from '@/components/ui/button';
import type { Thread } from '../data/schema';
import { useArchiveThread, useRetainThread, useUnarchiveThread, useUnretainThread } from '../data/threads';

export function ThreadActionButtons({ thread }: { thread: Thread }) {
  const { t } = useTranslation();
  const [showArchiveDialog, setShowArchiveDialog] = useState(false);
  const archiveMutation = useArchiveThread();
  const unarchiveMutation = useUnarchiveThread();
  const retainMutation = useRetainThread();
  const unretainMutation = useUnretainThread();
  const status = thread.status ?? 'active';

  return (
    <>
      <div className='flex items-center gap-1'>
        {status === 'active' && (
          <>
            <Button variant='ghost' size='sm' onClick={() => setShowArchiveDialog(true)} title={t('common.actions.archive')}>
              <IconArchive className='h-4 w-4' />
            </Button>
            <Button variant='ghost' size='sm' onClick={() => retainMutation.mutate(thread.id)} title={t('common.actions.retain')}>
              <IconPin className='h-4 w-4' />
            </Button>
          </>
        )}
        {status === 'archived' && (
          <Button variant='ghost' size='sm' onClick={() => unarchiveMutation.mutate(thread.id)} title={t('common.actions.unarchive')}>
            <IconRotate className='h-4 w-4' />
          </Button>
        )}
        {status === 'retained' && (
          <Button variant='ghost' size='sm' onClick={() => unretainMutation.mutate(thread.id)} title={t('common.actions.unretain')}>
            <IconRotate className='h-4 w-4' />
          </Button>
        )}
      </div>
      <AlertDialog open={showArchiveDialog} onOpenChange={setShowArchiveDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('threads.dialogs.archiveTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('threads.dialogs.archiveDescription')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.actions.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                archiveMutation.mutate(thread.id);
                setShowArchiveDialog(false);
              }}
            >
              {t('common.actions.archive')}
            </AlertDialogAction>
          </AlertDialogFooter>
        </AlertDialogContent>
      </AlertDialog>
    </>
  );
}
