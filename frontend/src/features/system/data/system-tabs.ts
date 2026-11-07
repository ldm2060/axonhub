// Route-facing system-tab helpers. Kept standalone (no imports from
// ../pages) because the route modules are part of the eager entry bundle and
// pages.ts pulls in every settings component.
export type SystemTabKey =
  | 'general'
  | 'security'
  | 'brand'
  | 'registration'
  | 'email'
  | 'about'
  | 'retry'
  | 'forwarding'
  | 'proxy'
  | 'quota'
  | 'streaming'
  | 'webhook'
  | 'claude-code'
  | 'codex'
  | 'storage'
  | 'diagnostics'
  | 'backup';

export const SYSTEM_TAB_KEYS: SystemTabKey[] = [
  'general',
  'security',
  'brand',
  'registration',
  'email',
  'about',
  'retry',
  'forwarding',
  'proxy',
  'quota',
  'streaming',
  'webhook',
  'claude-code',
  'codex',
  'storage',
  'diagnostics',
  'backup',
];

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

// Admin settings sub-pages (see ../pages). `general` lives at /admin/system.
const SYSTEM_SUB_PAGE_KEYS: ReadonlySet<string> = new Set(['requests', 'integrations', 'operations']);

export function isSystemSubPageKey(value: string | undefined): value is 'requests' | 'integrations' | 'operations' {
  return value !== undefined && SYSTEM_SUB_PAGE_KEYS.has(value);
}
