import { useCallback, useState } from 'react';
import type { Row } from '@tanstack/react-table';
import { Badge } from '@/components/ui/badge';
import { Switch } from '@/components/ui/switch';
import type { Prompt } from '../data/schema';
import { PromptsStatusDialog } from './prompts-status-dialog';

export function StatusSwitchCell({ row, canWrite }: { row: Row<Prompt>; canWrite: boolean }) {
  const prompt = row.original;
  const [dialogOpen, setDialogOpen] = useState(false);

  const isEnabled = prompt.status === 'enabled';

  const handleSwitchClick = useCallback(() => {
    setDialogOpen(true);
  }, []);

  if (!canWrite) {
    return <Badge variant='outline'>{prompt.status}</Badge>;
  }

  return (
    <>
      <Switch checked={isEnabled} onCheckedChange={handleSwitchClick} data-testid='prompt-status-switch' />
      {dialogOpen && <PromptsStatusDialog open={dialogOpen} onOpenChange={setDialogOpen} currentRow={prompt} />}
    </>
  );
}
