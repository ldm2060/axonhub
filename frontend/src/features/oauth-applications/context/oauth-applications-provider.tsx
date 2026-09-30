import { useState, type ReactNode } from 'react';
import type { OAuthApplication } from '../data/oauth-applications';
import { OAuthApplicationsContext } from './oauth-applications-context';

export default function OAuthApplicationsProvider({ children }: { children: ReactNode }) {
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);
  const [isEditDialogOpen, setIsEditDialogOpen] = useState(false);
  const [isDeleteDialogOpen, setIsDeleteDialogOpen] = useState(false);
  const [isRotateSecretDialogOpen, setIsRotateSecretDialogOpen] = useState(false);
  const [editingApplication, setEditingApplication] = useState<OAuthApplication | null>(null);
  const [deletingApplication, setDeletingApplication] = useState<OAuthApplication | null>(null);
  const [rotatingApplication, setRotatingApplication] = useState<OAuthApplication | null>(null);

  return (
    <OAuthApplicationsContext.Provider
      value={{
        isCreateDialogOpen,
        setIsCreateDialogOpen,
        isEditDialogOpen,
        setIsEditDialogOpen,
        isDeleteDialogOpen,
        setIsDeleteDialogOpen,
        isRotateSecretDialogOpen,
        setIsRotateSecretDialogOpen,
        editingApplication,
        setEditingApplication,
        deletingApplication,
        setDeletingApplication,
        rotatingApplication,
        setRotatingApplication,
      }}
    >
      {children}
    </OAuthApplicationsContext.Provider>
  );
}
