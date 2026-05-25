import * as React from 'react';
import { create } from 'zustand';

const PROJECT_STORAGE_KEY = 'axonhub_selected_project_id';

interface ProjectState {
  selectedProjectId: string | null;
  setSelectedProjectId: (projectId: string | null) => void;
  clearSelectedProjectId: () => void;
}

// Helper functions for localStorage
export const getProjectIdFromStorage = (): string | null => {
  try {
    return localStorage.getItem(PROJECT_STORAGE_KEY);
  } catch (_error) {
    return null;
  }
};

const setProjectIdToStorage = (projectId: string): void => {
  try {
    localStorage.setItem(PROJECT_STORAGE_KEY, projectId);
  } catch (_error) {
    // intentionally ignored
  }
};

const removeProjectIdFromStorage = (): void => {
  try {
    localStorage.removeItem(PROJECT_STORAGE_KEY);
  } catch (_error) {
    // intentionally ignored
  }
};

export const useProjectStore = create<ProjectState>()((set) => {
  const initProjectId = getProjectIdFromStorage();

  return {
    selectedProjectId: initProjectId,
    setSelectedProjectId: (projectId) => {
      set({ selectedProjectId: projectId });
      if (projectId) {
        setProjectIdToStorage(projectId);
      } else {
        removeProjectIdFromStorage();
      }
    },
    clearSelectedProjectId: () => {
      set({ selectedProjectId: null });
      removeProjectIdFromStorage();
    },
  };
});

// Convenience hook to get the selected project ID
export const useSelectedProjectId = () => useProjectStore((state) => state.selectedProjectId);

/**
 * Hook that auto-resolves the project context from the user's projects list.
 * Selects the first project if none is currently selected, or resets to the
 * first project if the current selection no longer exists.
 * This replaces the former ProjectSwitcher UI-driven selection.
 */
export function useAutoResolveProject(projects: { id: string }[] | undefined) {
  const { selectedProjectId, setSelectedProjectId } = useProjectStore();

  React.useEffect(() => {
    // If projects are still loading, do nothing
    if (!projects) {
      return;
    }

    // If user has no projects, clear the selection
    if (projects.length === 0) {
      if (selectedProjectId) {
        setSelectedProjectId(null);
      }
      return;
    }

    // If a project is selected and it still exists in the list, keep it
    if (selectedProjectId) {
      const projectExists = projects.some((p) => p.id === selectedProjectId);
      if (projectExists) {
        return;
      }
    }

    // Auto-select the first project
    const firstProject = projects[0];
    setSelectedProjectId(firstProject.id);
  }, [projects, selectedProjectId, setSelectedProjectId]);
}
