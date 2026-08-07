import { createContext, useContext, useState, type ReactNode } from 'react';
import type { UsageMonitorChannel } from '../data/schema';

type DialogType = 'add' | 'edit' | 'delete' | null;

interface UsageMonitorContextValue {
  open: DialogType;
  setOpen: (dialog: DialogType) => void;
  currentChannel: UsageMonitorChannel | null;
  setCurrentChannel: (channel: UsageMonitorChannel | null) => void;
}

const UsageMonitorContext = createContext<UsageMonitorContextValue | null>(null);

export default function UsageMonitorProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState<DialogType>(null);
  const [currentChannel, setCurrentChannel] = useState<UsageMonitorChannel | null>(null);

  return (
    <UsageMonitorContext.Provider value={{ open, setOpen, currentChannel, setCurrentChannel }}>{children}</UsageMonitorContext.Provider>
  );
}

export function useUsageMonitorContext() {
  const ctx = useContext(UsageMonitorContext);
  if (!ctx) throw new Error('useUsageMonitorContext must be used within UsageMonitorProvider');
  return ctx;
}
