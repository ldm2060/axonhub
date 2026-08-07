import { useState, useEffect } from 'react';
import { motion, AnimatePresence } from 'framer-motion';
import { ChevronDown } from 'lucide-react';

interface CollapsibleSectionProps {
  title: string;
  icon: React.ReactNode;
  children: React.ReactNode;
  storageKey: string;
  defaultOpen?: boolean;
}

export function CollapsibleSection({ title, icon, children, storageKey, defaultOpen = false }: CollapsibleSectionProps) {
  const [isOpen, setIsOpen] = useState(() => {
    try {
      const stored = localStorage.getItem(`dashboard-section-${storageKey}`);
      return stored !== null ? stored === 'true' : defaultOpen;
    } catch {
      return defaultOpen;
    }
  });

  useEffect(() => {
    try {
      localStorage.setItem(`dashboard-section-${storageKey}`, isOpen.toString());
    } catch {
      // Silently fail - persistence is a nice-to-have, not critical
    }
  }, [isOpen, storageKey]);

  return (
    <div className='space-y-4'>
      <button
        type='button'
        onClick={() => setIsOpen(!isOpen)}
        className='bg-card hover:bg-accent/50 flex w-full items-center justify-between rounded-lg border p-4 text-left transition-colors'
      >
        <div className='flex items-center gap-3'>
          <div className='bg-primary/10 flex h-8 w-8 items-center justify-center rounded-md'>{icon}</div>
          <span className='text-lg font-semibold'>{title}</span>
        </div>
        <motion.div animate={{ rotate: isOpen ? 180 : 0 }} transition={{ duration: 0.2, ease: 'easeInOut' }}>
          <ChevronDown className='text-muted-foreground h-5 w-5' />
        </motion.div>
      </button>
      <AnimatePresence initial={false}>
        {isOpen && (
          <motion.div
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.15, ease: 'easeInOut' }}
          >
            <div className='space-y-4'>{children}</div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
}
