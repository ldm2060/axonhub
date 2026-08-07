import { useCallback, useMemo, useState, type ReactNode } from 'react';
import type { PromptProtectionRule } from '../data/schema';
import { RulesContext, type RulesDialogType } from './rules-context';

export function PromptProtectionRulesProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState<RulesDialogType>(null);
  const [currentRow, setCurrentRow] = useState<PromptProtectionRule | null>(null);
  const [selectedRules, setSelectedRules] = useState<PromptProtectionRule[]>([]);
  const [resetRowSelection, setResetRowSelection] = useState<(() => void) | null>(null);

  const handleSetOpen = useCallback((nextOpen: RulesDialogType) => {
    setOpen(nextOpen);
    if (nextOpen !== 'edit' && nextOpen !== 'delete') {
      setCurrentRow(null);
    }
  }, []);

  const value = useMemo(
    () => ({
      open,
      setOpen: handleSetOpen,
      currentRow,
      setCurrentRow,
      selectedRules,
      setSelectedRules,
      resetRowSelection,
      setResetRowSelection,
    }),
    [open, handleSetOpen, currentRow, selectedRules, resetRowSelection]
  );

  return <RulesContext.Provider value={value}>{children}</RulesContext.Provider>;
}

export default PromptProtectionRulesProvider;
