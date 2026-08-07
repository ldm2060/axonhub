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
import type { Trace } from '../data/schema';
import { useArchiveTrace, useRetainTrace, useUnarchiveTrace, useUnretainTrace } from '../data/traces';

export function TraceActionButtons({ trace }: { trace: Trace }) {
  const { t } = useTranslation();
  const [showArchiveDialog, setShowArchiveDialog] = useState(false);
  const archiveMutation = useArchiveTrace();
  const unarchiveMutation = useUnarchiveTrace();
  const retainMutation = useRetainTrace();
  const unretainMutation = useUnretainTrace();
  const status = trace.status ?? 'active';

  return (
    <>
      <div className='flex items-center gap-1'>
        {status === 'active' && (
          <>
            <Button variant='ghost' size='sm' onClick={() => setShowArchiveDialog(true)} title={t('common.actions.archive')}>
              <IconArchive className='h-4 w-4' />
            </Button>
            <Button variant='ghost' size='sm' onClick={() => retainMutation.mutate(trace.id)} title={t('common.actions.retain')}>
              <IconPin className='h-4 w-4' />
            </Button>
          </>
        )}
        {status === 'archived' && (
          <Button variant='ghost' size='sm' onClick={() => unarchiveMutation.mutate(trace.id)} title={t('common.actions.unarchive')}>
            <IconRotate className='h-4 w-4' />
          </Button>
        )}
        {status === 'retained' && (
          <Button variant='ghost' size='sm' onClick={() => unretainMutation.mutate(trace.id)} title={t('common.actions.unretain')}>
            <IconRotate className='h-4 w-4' />
          </Button>
        )}
      </div>
      <AlertDialog open={showArchiveDialog} onOpenChange={setShowArchiveDialog}>
        <AlertDialogContent>
          <AlertDialogHeader>
            <AlertDialogTitle>{t('traces.dialogs.archiveTitle')}</AlertDialogTitle>
            <AlertDialogDescription>{t('traces.dialogs.archiveDescription')}</AlertDialogDescription>
          </AlertDialogHeader>
          <AlertDialogFooter>
            <AlertDialogCancel>{t('common.actions.cancel')}</AlertDialogCancel>
            <AlertDialogAction
              onClick={() => {
                archiveMutation.mutate(trace.id);
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
