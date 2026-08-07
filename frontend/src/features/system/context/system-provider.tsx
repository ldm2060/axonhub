import { useState, type ReactNode } from 'react';
import { SystemContext, type SystemContextType } from './system-context';

export default function SystemProvider({ children }: { children: ReactNode }) {
  const [isLoading, setIsLoading] = useState(false);

  const value: SystemContextType = {
    isLoading,
    setIsLoading,
  };

  return <SystemContext.Provider value={value}>{children}</SystemContext.Provider>;
}
