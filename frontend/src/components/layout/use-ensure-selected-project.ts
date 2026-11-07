import * as React from 'react';
import { useProjectStore } from '@/stores/projectStore';
import { useMyProjects } from '@/features/projects/data/projects';

// Keeps the selected project valid against the user's project list. Called from
// the layout so it runs even while the switcher is hidden outside the project module.
export function useEnsureSelectedProject() {
  const { data: myProjects } = useMyProjects();
  const { selectedProjectId, setSelectedProjectId } = useProjectStore();

  // 当项目列表加载完成后，验证并设置选中的项目
  React.useEffect(() => {
    // 如果项目列表还在加载，不做任何操作
    if (!myProjects) {
      return;
    }

    // 如果用户没有任何项目，清空选中的项目
    if (myProjects.length === 0) {
      if (selectedProjectId) {
        setSelectedProjectId(null);
      }
      return;
    }

    // 如果已有选中的项目且在列表中存在，保持选中状态
    if (selectedProjectId) {
      const projectExists = myProjects.some((p) => p.id === selectedProjectId);
      if (projectExists) {
        return;
      }
    }

    // 只有在以下情况才选择第一个项目：
    // 1. 没有选中的项目（首次访问）
    // 2. 选中的项目不在当前列表中（项目被删除或用户被移除）
    const firstProject = myProjects[0];
    setSelectedProjectId(firstProject.id);
  }, [myProjects, selectedProjectId, setSelectedProjectId]);
}
