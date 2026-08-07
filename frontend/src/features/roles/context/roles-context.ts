import { createContext, useContext } from 'react';
import type { Role } from '../data/schema';

export type RoleDialogType = 'create' | 'edit' | 'delete' | 'bulkDelete';

interface RolesContextType {
  editingRole: Role | null;
  setEditingRole: (role: Role | null) => void;
  deletingRole: Role | null;
  setDeletingRole: (role: Role | null) => void;
  selectedRoles: Role[];
  setSelectedRoles: (roles: Role[]) => void;
  isDialogOpen: Record<RoleDialogType, boolean>;
  openDialog: (type: RoleDialogType, role?: Role | Role[]) => void;
  closeDialog: (type?: RoleDialogType) => void;
  resetRowSelection: () => void;
  setResetRowSelection: (fn: () => void) => void;
}

export const RolesContext = createContext<RolesContextType | undefined>(undefined);

export function useRolesContext() {
  const context = useContext(RolesContext);
  if (!context) {
    throw new Error('useRolesContext must be used within a RolesProvider');
  }
  return context;
}
