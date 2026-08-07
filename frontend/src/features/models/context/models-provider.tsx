import { useCallback, useMemo, useState, type ReactNode } from 'react';
import type { Model } from '../data/schema';
import { ModelsContext, type ModelsDialogType } from './models-context';

export function ModelsProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState<ModelsDialogType>(null);
  const [currentRow, setCurrentRow] = useState<Model | null>(null);
  const [currentDeveloper, setCurrentDeveloper] = useState<string | null>(null);
  const [selectedModels, setSelectedModels] = useState<Model[]>([]);
  const [resetRowSelection, setResetRowSelection] = useState<(() => void) | null>(null);

  const handleSetOpen = useCallback((newOpen: ModelsDialogType) => {
    setOpen(newOpen);
  }, []);

  const handleSetCurrentRow = useCallback((row: Model | null) => {
    setCurrentRow(row);
  }, []);

  const handleSetCurrentDeveloper = useCallback((developer: string | null) => {
    setCurrentDeveloper(developer);
  }, []);

  const handleSetSelectedModels = useCallback((models: Model[]) => {
    setSelectedModels(models);
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
      currentDeveloper,
      setCurrentDeveloper: handleSetCurrentDeveloper,
      selectedModels,
      setSelectedModels: handleSetSelectedModels,
      resetRowSelection,
      setResetRowSelection: handleSetResetRowSelection,
    }),
    [
      open,
      handleSetOpen,
      currentRow,
      handleSetCurrentRow,
      currentDeveloper,
      handleSetCurrentDeveloper,
      selectedModels,
      handleSetSelectedModels,
      resetRowSelection,
      handleSetResetRowSelection,
    ]
  );

  return <ModelsContext.Provider value={value}>{children}</ModelsContext.Provider>;
}

export default ModelsProvider;
