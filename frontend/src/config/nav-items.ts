import {
  IconAB2,
  IconActivity,
  IconAi,
  IconBaselineDensityMedium,
  IconChartBar,
  IconDatabase,
  IconKey,
  IconLayoutDashboard,
  IconNote,
  IconRobot,
  IconSend,
  IconShield,
  IconUsers,
  IconUsersGroup,
} from '@tabler/icons-react';

// Structural definition of the sidebar navigation. It intentionally contains no
// translations or auth logic so it can be shared by:
// - `sidebar.ts` (rendering, after permission + visibility filtering)
// - the "customize menu" dialog (listing the items a user may hide)
// - the sign-in landing fallback (picking a visible default route)
export interface NavItemDef {
  // i18n key, e.g. "sidebar.items.dashboard".
  titleKey: string;
  // Stable route path, also used as the persisted identifier when hiding items.
  url: string;
  icon: React.ElementType;
  mobileOnly?: boolean;
}

export interface NavGroupDef {
  id: string;
  titleKey: string;
  items: NavItemDef[];
}

export const NAV_GROUP_DEFS: NavGroupDef[] = [
  {
    id: 'admin',
    titleKey: 'sidebar.groups.admin',
    items: [
      { titleKey: 'sidebar.items.dashboard', url: '/admin', icon: IconLayoutDashboard },
      { titleKey: 'sidebar.items.channels', url: '/admin/channels', icon: IconAi },
      { titleKey: 'sidebar.items.usageMonitor', url: '/admin/usage-monitor', icon: IconChartBar },
      { titleKey: 'sidebar.items.requests', url: '/admin/requests', icon: IconActivity },
      { titleKey: 'sidebar.items.models', url: '/admin/models', icon: IconRobot },
      { titleKey: 'sidebar.items.publishRequests', url: '/admin/publish-requests', icon: IconSend },
      { titleKey: 'sidebar.items.promptProtectionRules', url: '/admin/prompt-protection-rules', icon: IconShield },
      { titleKey: 'sidebar.items.dataStorages', url: '/admin/data-storages', icon: IconDatabase },
      { titleKey: 'sidebar.items.users', url: '/admin/users', icon: IconUsers },
      { titleKey: 'sidebar.items.roles', url: '/admin/roles', icon: IconUsersGroup },
      { titleKey: 'sidebar.items.system', url: '/admin/runtime', icon: IconActivity },
    ],
  },
  {
    id: 'personal',
    titleKey: 'sidebar.groups.personal',
    items: [
      { titleKey: 'sidebar.items.dashboard', url: '/', icon: IconLayoutDashboard },
      { titleKey: 'sidebar.items.myChannels', url: '/my-channels', icon: IconAi },
      { titleKey: 'sidebar.items.myModels', url: '/my-models', icon: IconRobot },
      { titleKey: 'sidebar.items.apiKeys', url: '/project/api-keys', icon: IconKey },
      { titleKey: 'sidebar.items.prompts', url: '/project/prompts', icon: IconNote },
      { titleKey: 'sidebar.items.requests', url: '/project/requests', icon: IconActivity },
      { titleKey: 'sidebar.items.traces', url: '/project/traces', icon: IconAB2 },
      { titleKey: 'sidebar.items.threads', url: '/project/threads', icon: IconBaselineDensityMedium },
    ],
  },
];

// All navigable URLs in display order.
export const NAV_ITEM_URLS: string[] = NAV_GROUP_DEFS.flatMap((group) => group.items.map((item) => item.url));

// Personal (non-admin) URLs, used as the landing fallback for non-owner users.
export const PERSONAL_NAV_ITEM_URLS: string[] = (NAV_GROUP_DEFS.find((group) => group.id === 'personal')?.items ?? []).map(
  (item) => item.url
);

// Pick a landing route that is not hidden. Falls back to `preferred` when the
// user has hidden every candidate.
export function pickFallbackNavUrl(preferred: string, hiddenItems: string[], isOwner: boolean): string {
  if (!hiddenItems.includes(preferred)) {
    return preferred;
  }

  const candidates = isOwner ? NAV_ITEM_URLS : PERSONAL_NAV_ITEM_URLS;

  return candidates.find((url) => !hiddenItems.includes(url)) ?? preferred;
}
