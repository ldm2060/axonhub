'use client';

import { type ComponentProps } from 'react';
import type { ToolUIPart } from 'ai';
import { cn } from '@/lib/utils';
import { Alert } from '@/components/ui/alert';

export type ConfirmationProps = ComponentProps<typeof Alert> & {
  state: ToolUIPart['state'];
};

export const Confirmation = ({ className, state, ...props }: ConfirmationProps) => {
  if (state === 'input-streaming' || state === 'input-available') {
    return null;
  }

  return <Alert className={cn('flex flex-col gap-2', className)} {...props} />;
};
