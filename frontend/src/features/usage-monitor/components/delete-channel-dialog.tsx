'use client';

import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { useUsageMonitorContext } from '../context/usage-monitor-context';
import { useDeleteUsageMonitorChannel } from '../data/usage-monitor';

export function DeleteChannelDialog() {
  const { t } = useTranslation();
  const { open, setOpen, currentChannel } = useUsageMonitorContext();
  const deleteMutation = useDeleteUsageMonitorChannel();

  const isOpen = open === 'delete';

  async function handleDelete() {
    if (!currentChannel) return;
    try {
      await deleteMutation.mutateAsync(currentChannel.id);
      setOpen(null);
    } catch {
      // error handled by mutation
    }
  }

  return (
    <Dialog
      open={isOpen}
      onOpenChange={(v) => {
        if (!v) setOpen(null);
      }}
    >
      <DialogContent className='sm:max-w-md'>
        <DialogHeader>
          <DialogTitle>{t('usageMonitor.deleteChannel')}</DialogTitle>
          <DialogDescription>{t('usageMonitor.deleteConfirm')}</DialogDescription>
        </DialogHeader>

        {currentChannel && (
          <p className='text-sm'>
            {t('usageMonitor.deleteConfirm')} <strong>{currentChannel.name}</strong>
          </p>
        )}

        <DialogFooter>
          <Button type='button' variant='outline' onClick={() => setOpen(null)}>
            {t('common.buttons.cancel')}
          </Button>
          <Button type='button' variant='destructive' onClick={handleDelete} disabled={deleteMutation.isPending}>
            {deleteMutation.isPending ? t('common.buttons.processing') : t('common.buttons.delete')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
