import { useEffect } from 'react';
import { useRouter } from '@tanstack/react-router';
import { useSelectedProjectId } from '@/stores/projectStore';
import { useMyProjects } from '@/features/projects/data/projects';

interface ProjectGuardProps {
  children: React.ReactNode;
  fallbackPath?: string;
}

/**
 * ProjectGuard ensures a project context is available before rendering children.
 * With auto-resolved project context (via useAutoResolveProject in AppHeader),
 * this guard only needs to handle the loading state and the edge case where
 * the user has no projects at all.
 */
export function ProjectGuard({ children, fallbackPath = '/projects' }: ProjectGuardProps) {
  const router = useRouter();
  const selectedProjectId = useSelectedProjectId();
  const { data: myProjects, isLoading } = useMyProjects();

  useEffect(() => {
    // Once loading completes, if there's no selected project and no projects available,
    // redirect to the projects management page
    if (!isLoading && !selectedProjectId && (!myProjects || myProjects.length === 0)) {
      router.navigate({ to: fallbackPath });
    }
  }, [selectedProjectId, isLoading, myProjects, fallbackPath, router]);

  // Loading or waiting for auto-resolution
  if (isLoading) {
    return null;
  }

  // No project context yet (auto-resolution hasn't kicked in, or user has no projects)
  if (!selectedProjectId) {
    return null;
  }

  return <>{children}</>;
}
