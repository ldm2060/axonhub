import { useCallback, useState } from 'react';
import type { Row } from '@tanstack/react-table';
import { Switch } from '@/components/ui/switch';
import type { PromptProtectionRule } from '../data/schema';
import { RulesStatusDialog } from './rules-status-dialog';

export function StatusSwitchCell({ row }: { row: Row<PromptProtectionRule> }) {
  const rule = row.original;
  const [dialogOpen, setDialogOpen] = useState(false);
  const isEnabled = rule.status === 'enabled';

  const handleSwitchClick = useCallback(() => {
    setDialogOpen(true);
  }, []);

  return (
    <>
      <Switch checked={isEnabled} onCheckedChange={handleSwitchClick} />
      {dialogOpen && <RulesStatusDialog open={dialogOpen} onOpenChange={setDialogOpen} currentRow={rule} />}
    </>
  );
}
