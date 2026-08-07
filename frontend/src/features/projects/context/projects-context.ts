import { createContext, useContext } from 'react';
import type { Project } from '../data/schema';

interface ProjectsContextType {
  editingProject: Project | null;
  setEditingProject: (project: Project | null) => void;
  archivingProject: Project | null;
  setArchivingProject: (project: Project | null) => void;
  activatingProject: Project | null;
  setActivatingProject: (project: Project | null) => void;
  deletingProject: Project | null;
  setDeletingProject: (project: Project | null) => void;
  profilesProject: Project | null;
  setProfilesProject: (project: Project | null) => void;
  isCreateDialogOpen: boolean;
  setIsCreateDialogOpen: (open: boolean) => void;
}

export const ProjectsContext = createContext<ProjectsContextType | undefined>(undefined);

export function useProjectsContext() {
  const context = useContext(ProjectsContext);
  if (!context) {
    throw new Error('useProjectsContext must be used within a ProjectsProvider');
  }
  return context;
}
