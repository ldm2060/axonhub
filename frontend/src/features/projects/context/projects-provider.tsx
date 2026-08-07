import { useState, type ReactNode } from 'react';
import type { Project } from '../data/schema';
import { ProjectsContext } from './projects-context';

export default function ProjectsProvider({ children }: { children: ReactNode }) {
  const [editingProject, setEditingProject] = useState<Project | null>(null);
  const [archivingProject, setArchivingProject] = useState<Project | null>(null);
  const [activatingProject, setActivatingProject] = useState<Project | null>(null);
  const [deletingProject, setDeletingProject] = useState<Project | null>(null);
  const [profilesProject, setProfilesProject] = useState<Project | null>(null);
  const [isCreateDialogOpen, setIsCreateDialogOpen] = useState(false);

  return (
    <ProjectsContext.Provider
      value={{
        editingProject,
        setEditingProject,
        archivingProject,
        setArchivingProject,
        activatingProject,
        setActivatingProject,
        deletingProject,
        setDeletingProject,
        profilesProject,
        setProfilesProject,
        isCreateDialogOpen,
        setIsCreateDialogOpen,
      }}
    >
      {children}
    </ProjectsContext.Provider>
  );
}
