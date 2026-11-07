import type { SVGProps } from 'react';
import { cn } from '@/lib/utils';
import { AXONHUB_LOGO_ARCS, AXONHUB_LOGO_NODES } from './axonhub-logo-geometry';

export function AxonHubLogo({ className, ...props }: SVGProps<SVGSVGElement>) {
  return (
    <svg viewBox='0 0 64 64' fill='none' aria-label='AxonHub' role='img' className={cn('size-8', className)} {...props}>
      <path d={AXONHUB_LOGO_ARCS} stroke='currentColor' strokeWidth='3.5' strokeLinecap='round' strokeLinejoin='round' />
      {AXONHUB_LOGO_NODES.base.map(([cx, cy]) => (
        <circle key={`${cx}-${cy}`} cx={cx} cy={cy} r='5.5' fill='currentColor' />
      ))}
      {AXONHUB_LOGO_NODES.accent.map(([cx, cy]) => (
        <circle key={`${cx}-${cy}`} cx={cx} cy={cy} r='5.5' fill='var(--logo-accent, #A8844E)' />
      ))}
    </svg>
  );
}
