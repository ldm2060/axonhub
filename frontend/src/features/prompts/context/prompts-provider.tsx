import { useCallback, useMemo, useState, type ReactNode } from 'react';
import type { Prompt } from '../data/schema';
import { PromptsContext, type PromptsDialogType } from './prompts-context';

export function PromptsProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState<PromptsDialogType>(null);
  const [currentRow, setCurrentRow] = useState<Prompt | null>(null);
  const [selectedPrompts, setSelectedPrompts] = useState<Prompt[]>([]);
  const [resetRowSelection, setResetRowSelection] = useState<(() => void) | null>(null);

  const handleSetOpen = useCallback((newOpen: PromptsDialogType) => {
    setOpen(newOpen);
  }, []);

  const handleSetCurrentRow = useCallback((row: Prompt | null) => {
    setCurrentRow(row);
  }, []);

  const handleSetSelectedPrompts = useCallback((prompts: Prompt[]) => {
    setSelectedPrompts(prompts);
  }, []);

  const handleSetResetRowSelection = useCallback((fn: (() => void) | null) => {
    setResetRowSelection(() => fn);
  }, []);

  const value = useMemo(
    () => ({
      open,
      setOpen: handleSetOpen,
      currentRow,
      setCurrentRow: handleSetCurrentRow,
      selectedPrompts,
      setSelectedPrompts: handleSetSelectedPrompts,
      resetRowSelection,
      setResetRowSelection: handleSetResetRowSelection,
    }),
    [
      open,
      handleSetOpen,
      currentRow,
      handleSetCurrentRow,
      selectedPrompts,
      handleSetSelectedPrompts,
      resetRowSelection,
      handleSetResetRowSelection,
    ]
  );

  return <PromptsContext.Provider value={value}>{children}</PromptsContext.Provider>;
}

export default PromptsProvider;
