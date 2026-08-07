import { createContext, useContext } from 'react';
import type { Role } from '../data/schema';

interface RolesContextType {
  editingRole: Role | null;
  setEditingRole: (role: Role | null) => void;
  deletingRole: Role | null;
  setDeletingRole: (role: Role | null) => void;
  isCreateDialogOpen: boolean;
  setIsCreateDialogOpen: (open: boolean) => void;
}

export const RolesContext = createContext<RolesContextType | undefined>(undefined);

export function useRolesContext() {
  const context = useContext(RolesContext);
  if (!context) {
    throw new Error('useRolesContext must be used within a RolesProvider');
  }
  return context;
}
