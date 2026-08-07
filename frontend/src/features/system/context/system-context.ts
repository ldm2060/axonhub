import { createContext, useContext } from 'react';

export interface SystemContextType {
  isLoading: boolean;
  setIsLoading: (loading: boolean) => void;
}

export const SystemContext = createContext<SystemContextType | undefined>(undefined);

export function useSystemContext() {
  const context = useContext(SystemContext);
  if (!context) {
    throw new Error('useSystemContext must be used within a SystemProvider');
  }
  return context;
}
