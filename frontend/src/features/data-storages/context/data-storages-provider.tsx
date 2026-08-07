import { useState, type ReactNode } from 'react';
import type { DataStorage } from '../data/data-storages';
import { DataStoragesContext } from './data-storages-context';

export default function DataStoragesProvider({ children }: { children: ReactNode }) {
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false);
  const [isArchiveDialogOpen, setIsArchiveDialogOpen] = useState(false);
  const [editingDataStorage, setEditingDataStorage] = useState<DataStorage | null>(null);
  const [archiveDataStorage, setArchiveDataStorage] = useState<DataStorage | null>(null);

  return (
    <DataStoragesContext.Provider
      value={{
        isCreateDialogOpen,
        setIsCreateDialogOpen,
        isEditDialogOpen,
        setIsEditDialogOpen,
        isArchiveDialogOpen,
        setIsArchiveDialogOpen,
        editingDataStorage,
        setEditingDataStorage,
        archiveDataStorage,
        setArchiveDataStorage,
      }}
    >
      {children}
    </DataStoragesContext.Provider>
  );
}
