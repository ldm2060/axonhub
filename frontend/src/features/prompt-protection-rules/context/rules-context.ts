import { createContext, useContext } from 'react';
import type { PromptProtectionRule } from '../data/schema';

export type RulesDialogType = 'create' | 'edit' | 'delete' | 'bulkEnable' | 'bulkDisable' | 'bulkDelete' | null;

interface RulesContextType {
  open: RulesDialogType;
  setOpen: (open: RulesDialogType) => void;
  currentRow: PromptProtectionRule | null;
  setCurrentRow: (row: PromptProtectionRule | null) => void;
  selectedRules: PromptProtectionRule[];
  setSelectedRules: (rules: PromptProtectionRule[]) => void;
  resetRowSelection: (() => void) | null;
  setResetRowSelection: (fn: (() => void) | null) => void;
}

export const RulesContext = createContext<RulesContextType | undefined>(undefined);

export function usePromptProtectionRules() {
  const context = useContext(RulesContext);
  if (!context) {
    throw new Error('usePromptProtectionRules must be used within PromptProtectionRulesProvider');
  }

  return context;
}
