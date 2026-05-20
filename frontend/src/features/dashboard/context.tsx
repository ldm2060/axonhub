import { createContext, useContext } from 'react';
import type { DashboardMode } from '../data/dashboard';

const DashboardModeContext = createContext<DashboardMode>('project');

export function useDashboardMode(): DashboardMode {
  return useContext(DashboardModeContext);
}

export { DashboardModeContext };
