import { createContext, useContext } from 'react';
import type { Prompt } from '../data/schema';

export type PromptsDialogType = 'create' | 'edit' | 'delete' | 'bulkEnable' | 'bulkDisable' | 'bulkDelete' | null;

interface PromptsContextType {
  open: PromptsDialogType;
  setOpen: (open: PromptsDialogType) => void;
  currentRow: Prompt | null;
  setCurrentRow: (row: Prompt | null) => void;
  selectedPrompts: Prompt[];
  setSelectedPrompts: (prompts: Prompt[]) => void;
  resetRowSelection: (() => void) | null;
  setResetRowSelection: (fn: (() => void) | null) => void;
}

export const PromptsContext = createContext<PromptsContextType | undefined>(undefined);

export function usePrompts() {
  const context = useContext(PromptsContext);
  if (context === undefined) {
    throw new Error('usePrompts must be used within a PromptsProvider');
  }
  return context;
}
