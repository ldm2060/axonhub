import { createContext, useContext } from 'react';
import type { OAuthApplication } from '../data/oauth-applications';

interface OAuthApplicationsContextType {
  isCreateDialogOpen: boolean;
  setIsCreateDialogOpen: (open: boolean) => void;
  isEditDialogOpen: boolean;
  setIsEditDialogOpen: (open: boolean) => void;
  isDeleteDialogOpen: boolean;
  setIsDeleteDialogOpen: (open: boolean) => void;
  isRotateSecretDialogOpen: boolean;
  setIsRotateSecretDialogOpen: (open: boolean) => void;
  editingApplication: OAuthApplication | null;
  setEditingApplication: (application: OAuthApplication | null) => void;
  deletingApplication: OAuthApplication | null;
  setDeletingApplication: (application: OAuthApplication | null) => void;
  rotatingApplication: OAuthApplication | null;
  setRotatingApplication: (application: OAuthApplication | null) => void;
}

export const OAuthApplicationsContext = createContext<OAuthApplicationsContextType | undefined>(undefined);

export function useOAuthApplicationsContext() {
  const context = useContext(OAuthApplicationsContext);
  if (!context) {
    throw new Error('useOAuthApplicationsContext must be used within OAuthApplicationsProvider');
  }
  return context;
}
