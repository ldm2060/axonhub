import { useState, type ReactNode } from 'react';
import type { Role } from '../data/schema';
import { RolesContext } from './roles-context';

export default function RolesProvider({ children }: { children: ReactNode }) {
  const [editingRole, setEditingRole] = useState<Role | null>(null);
  const [deletingRole, setDeletingRole] = useState<Role | null>(null);
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);

  return (
    <RolesContext.Provider
      value={{
        editingRole,
        setEditingRole,
        deletingRole,
        setDeletingRole,
        isCreateDialogOpen,
        setIsCreateDialogOpen,
      }}
    >
      {children}
    </RolesContext.Provider>
  );
}
