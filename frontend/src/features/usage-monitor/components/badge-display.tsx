import { resolveBadgeGradient, BADGE_GRADIENTS } from './badge-styles';

interface BadgeDisplayProps {
  value: string;
  badge?: string;
  badgePresets?: string;
}

export function BadgeDisplay({ value, badge, badgePresets }: BadgeDisplayProps) {
  const gradientKey = resolveBadgeGradient(badge, badgePresets, value);
  if (!gradientKey) {
    return <span>{value}</span>;
  }
  const gradient = BADGE_GRADIENTS[gradientKey];
  return (
    <span
      className='inline-flex items-center rounded-md px-2.5 py-0.5 text-xs font-semibold tracking-wide uppercase'
      style={{
        background: gradient.css,
        color: gradient.textColor,
      }}
    >
      {value}
    </span>
  );
}
