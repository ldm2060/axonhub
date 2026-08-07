import { createContext, useContext } from 'react';
import type { UsageMonitorChannel } from '../data/schema';

export type UsageMonitorDialogType = 'add' | 'edit' | 'delete' | null;

interface UsageMonitorContextValue {
  open: UsageMonitorDialogType;
  setOpen: (dialog: UsageMonitorDialogType) => void;
  currentChannel: UsageMonitorChannel | null;
  setCurrentChannel: (channel: UsageMonitorChannel | null) => void;
}

export const UsageMonitorContext = createContext<UsageMonitorContextValue | null>(null);

export function useUsageMonitorContext() {
  const context = useContext(UsageMonitorContext);
  if (!context) throw new Error('useUsageMonitorContext must be used within UsageMonitorProvider');
  return context;
}
