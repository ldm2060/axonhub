export const SYSTEM_TAB_KEYS = [
  'general',
  'security',
  'brand',
  'registration',
  'email',
  'storage',
  'retry',
  'streaming',
  'webhook',
  'proxy',
  'quota',
  'backup',
  'diagnostics',
  'about',
] as const;

export type SystemTabKey = (typeof SYSTEM_TAB_KEYS)[number];

const SYSTEM_TAB_KEY_SET: ReadonlySet<string> = new Set(SYSTEM_TAB_KEYS);

export function isSystemTabKey(value: unknown): value is SystemTabKey {
  return typeof value === 'string' && SYSTEM_TAB_KEY_SET.has(value);
}

export const DEFAULT_SYSTEM_TAB: SystemTabKey = 'general';
export const OWNER_ONLY_SYSTEM_TABS: ReadonlySet<SystemTabKey> = new Set(['registration', 'email', 'backup', 'diagnostics']);

export function resolveSystemTab(value: unknown, isOwner: boolean): SystemTabKey {
  if (!isSystemTabKey(value) || (!isOwner && OWNER_ONLY_SYSTEM_TABS.has(value))) {
    return DEFAULT_SYSTEM_TAB;
  }
  return value;
}
