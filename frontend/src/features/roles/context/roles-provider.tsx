import { useRef, useState, type ReactNode } from 'react';
import type { Role } from '../data/schema';
import { RolesContext, type RoleDialogType } from './roles-context';

export default function RolesProvider({ children }: { children: ReactNode }) {
  const [editingRole, setEditingRole] = useState<Role | null>(null);
  const [deletingRole, setDeletingRole] = useState<Role | null>(null);
  const [selectedRoles, setSelectedRoles] = useState<Role[]>([]);
  const [isDialogOpen, setIsDialogOpen] = useState<Record<RoleDialogType, boolean>>({
    create: false,
    edit: false,
    delete: false,
    bulkDelete: false,
  });
  const resetRowSelectionRef = useRef<() => void>(() => {});

  const openDialog = (type: RoleDialogType, role?: Role | Role[]) => {
    if (role) {
      if (Array.isArray(role)) {
        setSelectedRoles(role);
      } else {
        setEditingRole(role);
        setDeletingRole(role);
      }
    }
    setIsDialogOpen((prev) => ({ ...prev, [type]: true }));
  };

  const closeDialog = (type?: RoleDialogType) => {
    if (type) {
      setIsDialogOpen((prev) => ({ ...prev, [type]: false }));
      if (type === 'delete' || type === 'edit') {
        setEditingRole(null);
        setDeletingRole(null);
      }
      if (type === 'bulkDelete') {
        setSelectedRoles([]);
      }
    } else {
      setIsDialogOpen({
        create: false,
        edit: false,
        delete: false,
        bulkDelete: false,
      });
      setEditingRole(null);
      setDeletingRole(null);
      setSelectedRoles([]);
    }
  };

  return (
    <RolesContext.Provider
      value={{
        editingRole,
        setEditingRole,
        deletingRole,
        setDeletingRole,
        selectedRoles,
        setSelectedRoles,
        isDialogOpen,
        openDialog,
        closeDialog,
        resetRowSelection: () => resetRowSelectionRef.current(),
        setResetRowSelection: (fn: () => void) => {
          resetRowSelectionRef.current = fn;
        },
      }}
    >
      {children}
    </RolesContext.Provider>
  );
}
