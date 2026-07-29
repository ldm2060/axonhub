import {
  IconLayoutDashboard,
  IconRobot,
  IconShield,
  IconKey,
  IconActivity,
  IconDatabase,
  IconAB2,
  IconBaselineDensityMedium,
  IconAi,
  IconNote,
  IconSend,
  IconUsers,
  IconUsersGroup,
  IconChartBar,
  IconServer,
} from '@tabler/icons-react';
import { Command } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '@/stores/authStore';
import { useRoutePermissions } from '@/hooks/useRoutePermissions';
import { useMe } from '@/features/auth/data/auth';
import { type SidebarData, type NavGroup, type NavLink } from './components/layout/types';

export function useSidebarData(): SidebarData {
  const { t, i18n } = useTranslation();
  const { user: authUser } = useAuthStore((state) => state.auth);
  const { data: meData } = useMe();
  const { filterNavGroups } = useRoutePermissions();

  // Use data from me query if available, otherwise fall back to auth store
  const user = meData || authUser;

  // Generate user initials for avatar
  const getInitials = (firstName?: string, lastName?: string, email?: string) => {
    if (firstName && lastName) {
      const isZh = i18n.language?.startsWith('zh');
      const [first, second] = isZh ? [lastName, firstName] : [firstName, lastName];
      return `${first.charAt(0)}${second.charAt(0)}`.toUpperCase();
    }
    if (firstName) {
      return firstName.slice(0, 2).toUpperCase();
    }
    if (email) {
      return email.split('@')[0].slice(0, 2).toUpperCase();
    }
    return 'U';
  };

  // Generate user display name
  const getDisplayName = (firstName?: string, lastName?: string, email?: string) => {
    if (firstName && lastName) {
      const isZh = i18n.language?.startsWith('zh');
      return isZh ? `${lastName} ${firstName}` : `${firstName} ${lastName}`;
    }
    if (firstName) {
      return firstName;
    }
    if (email) {
      const username = email.split('@')[0];
      return username.charAt(0).toUpperCase() + username.slice(1);
    }
    return 'User';
  };

  // 原始导航组配置
  const rawNavGroups: NavGroup[] = [
    {
      title: t('sidebar.groups.admin'),
      items: [
        {
          title: t('sidebar.items.dashboard'),
          url: '/admin',
          icon: IconLayoutDashboard,
        } as NavLink,
        {
          title: t('sidebar.items.channels'),
          url: '/admin/channels',
          icon: IconAi,
        } as NavLink,
        {
          title: t('sidebar.items.usageMonitor'),
          url: '/admin/usage-monitor',
          icon: IconChartBar,
        } as NavLink,
        {
          title: t('sidebar.items.requests'),
          url: '/admin/requests',
          icon: IconActivity,
        } as NavLink,
        {
          title: t('sidebar.items.models'),
          url: '/admin/models',
          icon: IconRobot,
        } as NavLink,
        {
          title: t('sidebar.items.publishRequests'),
          url: '/admin/publish-requests',
          icon: IconSend,
        } as NavLink,
        {
          title: t('sidebar.items.promptProtectionRules'),
          url: '/admin/prompt-protection-rules',
          icon: IconShield,
        } as NavLink,
        {
          title: t('sidebar.items.dataStorages'),
          url: '/admin/data-storages',
          icon: IconDatabase,
        } as NavLink,
        {
          title: t('sidebar.items.users'),
          url: '/admin/users',
          icon: IconUsers,
        } as NavLink,
        {
          title: t('sidebar.items.roles'),
          url: '/admin/roles',
          icon: IconUsersGroup,
        } as NavLink,
        {
          title: t('sidebar.items.system'),
          url: '/admin/system',
          icon: IconServer,
        } as NavLink,
      ],
    },
    {
      title: t('sidebar.groups.personal'),
      items: [
        {
          title: t('sidebar.items.dashboard'),
          url: '/',
          icon: IconLayoutDashboard,
        } as NavLink,
        {
          title: t('sidebar.items.myChannels'),
          url: '/my-channels',
          icon: IconAi,
        } as NavLink,
        {
          title: t('sidebar.items.myModels'),
          url: '/my-models',
          icon: IconRobot,
        } as NavLink,
        {
          title: t('sidebar.items.apiKeys'),
          url: '/project/api-keys',
          icon: IconKey,
        } as NavLink,
        {
          title: t('sidebar.items.prompts'),
          url: '/project/prompts',
          icon: IconNote,
        } as NavLink,
        {
          title: t('sidebar.items.requests'),
          url: '/project/requests',
          icon: IconActivity,
        } as NavLink,
        {
          title: t('sidebar.items.traces'),
          url: '/project/traces',
          icon: IconAB2,
        } as NavLink,
        {
          title: t('sidebar.items.threads'),
          url: '/project/threads',
          icon: IconBaselineDensityMedium,
        } as NavLink,
      ],
    },
  ];

  // 使用权限过滤导航组
  const filteredNavGroups = filterNavGroups(rawNavGroups);

  // Admin group only visible to system owners
  const isSystemOwner = user?.isOwner === true;
  const finalNavGroups = filteredNavGroups
    .map((group) => {
      if (group.title === t('sidebar.groups.admin') && !isSystemOwner) {
        return { ...group, items: [] };
      }
      return group;
    })
    .filter((group) => group.items.length > 0);

  return {
    user: {
      name: getDisplayName(user?.firstName, user?.lastName, user?.email),
      email: user?.email || 'user@example.com',
      avatar: user?.avatar || getInitials(user?.firstName, user?.lastName, user?.email),
    },
    teams: [
      {
        name: t('sidebar.team.name'),
        logo: Command,
        description: '',
      },
    ],
    navGroups: finalNavGroups,
  };
}
