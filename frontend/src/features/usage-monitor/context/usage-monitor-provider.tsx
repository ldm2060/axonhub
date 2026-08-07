import { useState, type ReactNode } from 'react';
import type { UsageMonitorChannel } from '../data/schema';
import { UsageMonitorContext, type UsageMonitorDialogType } from './usage-monitor-context';

export default function UsageMonitorProvider({ children }: { children: ReactNode }) {
  const [open, setOpen] = useState<UsageMonitorDialogType>(null);
  const [currentChannel, setCurrentChannel] = useState<UsageMonitorChannel | null>(null);

  return (
    <UsageMonitorContext.Provider value={{ open, setOpen, currentChannel, setCurrentChannel }}>{children}</UsageMonitorContext.Provider>
  );
}
