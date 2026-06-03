export type BadgeGradientKey = 'sapphire' | 'rosegold' | 'champagne' | 'freshgreen' | 'amber' | 'default';

export const BADGE_GRADIENTS: Record<BadgeGradientKey, { css: string; textColor: string; label: string }> = {
  sapphire: {
    css: 'linear-gradient(135deg, #3498DB 0%, #0F52BA 50%, #1B4F72 100%)',
    textColor: '#ffffff',
    label: 'Sapphire Blue',
  },
  rosegold: {
    css: 'linear-gradient(135deg, #B76E79 0%, #E0A899 35%, #F9E4B7 65%, #B76E79 100%)',
    textColor: '#3d2020',
    label: 'Rose Gold',
  },
  champagne: {
    css: 'linear-gradient(135deg, #C5B358 0%, #E7D3B5 40%, #FFF8DC 70%, #D4AF37 100%)',
    textColor: '#3d3520',
    label: 'Champagne Gold',
  },
  freshgreen: {
    css: 'linear-gradient(135deg, #E8F8F5 0%, #A3E4D7 50%, #48C9B0 100%)',
    textColor: '#1a4a3a',
    label: 'Fresh Green',
  },
  amber: {
    css: 'linear-gradient(135deg, #F59E0B 0%, #D97706 50%, #92400E 100%)',
    textColor: '#ffffff',
    label: 'Amber',
  },
  default: {
    css: 'linear-gradient(135deg, #f0f0f0 0%, #e0e0e0 100%)',
    textColor: '#333333',
    label: 'Default',
  },
};

export function resolveBadgeGradient(
  badge: string | undefined,
  badgePresets: string | undefined,
  value: string | null | undefined,
): BadgeGradientKey | null {
  if (!badge || !badgePresets || !value) return null;
  try {
    const presets = JSON.parse(badgePresets) as Record<string, BadgeGradientKey>;
    return presets[value.toLowerCase()] ?? null;
  } catch {
    return null;
  }
}
