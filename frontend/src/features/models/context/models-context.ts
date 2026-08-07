import { createContext, useContext } from 'react';
import type { Model } from '../data/schema';

export type ModelsDialogType =
  | 'create'
  | 'batchCreate'
  | 'edit'
  | 'delete'
  | 'archive'
  | 'association'
  | 'developerAssociation'
  | 'settings'
  | 'bulkEnable'
  | 'bulkDisable'
  | 'unassociated'
  | null;

interface ModelsContextType {
  open: ModelsDialogType;
  setOpen: (open: ModelsDialogType) => void;
  currentRow: Model | null;
  setCurrentRow: (row: Model | null) => void;
  currentDeveloper: string | null;
  setCurrentDeveloper: (developer: string | null) => void;
  selectedModels: Model[];
  setSelectedModels: (models: Model[]) => void;
  resetRowSelection: (() => void) | null;
  setResetRowSelection: (fn: (() => void) | null) => void;
}

export const ModelsContext = createContext<ModelsContextType | undefined>(undefined);

export function useModels() {
  const context = useContext(ModelsContext);
  if (context === undefined) {
    throw new Error('useModels must be used within a ModelsProvider');
  }
  return context;
}
