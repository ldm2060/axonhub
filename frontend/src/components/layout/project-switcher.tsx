import { useAutoResolveProject } from '@/stores/projectStore';
import { useMyProjects } from '@/features/projects/data/projects';

/**
 * ProjectAutoResolver is a non-UI component that auto-resolves the project context.
 * It replaces the former ProjectSwitcher dropdown with automatic selection.
 * Place this in the app layout to ensure project context is always available.
 */
export function ProjectAutoResolver() {
  const { data: myProjects } = useMyProjects();
  useAutoResolveProject(myProjects);
  return null;
}
