// 路由权限配置
export type ScopeLevel = 'system' | 'project' | 'any';

export interface RouteConfig {
  path: string;
  requiredScopes?: string[];
  scopeLevel?: ScopeLevel; // 权限级别：system 只检查系统级权限，project 只检查项目级权限，any 检查两者
  mode?: 'hidden' | 'disabled'; // 当没有权限时的处理方式
  children?: RouteConfig[];
  requireProjectOwner?: boolean; // 是否需要 project owner (或 system owner)
}

export interface RouteGroup {
  title: string;
  scopeLevel?: ScopeLevel; // 路由组的默认权限级别
  routes: RouteConfig[];
}

// 定义所有路由的权限配置
export const routeConfigs: RouteGroup[] = [
  {
    title: 'Admin',
    scopeLevel: 'system', // Admin 路由组只能通过 system-level 权限访问
    routes: [
      {
        path: '/admin',
        requiredScopes: ['read_dashboard'],
        mode: 'hidden',
      },
      {
        path: '/admin/channels',
        requiredScopes: ['read_channels'],
        mode: 'hidden',
      },
      {
        path: '/admin/usage-monitor',
        requiredScopes: ['read_channels'],
        mode: 'hidden',
      },
      {
        path: '/admin/requests',
        requiredScopes: ['read_requests'],
        mode: 'hidden',
      },
      {
        path: '/admin/models',
        requiredScopes: ['read_channels'],
        mode: 'hidden',
      },
      {
        path: '/admin/publish-requests',
        requiredScopes: ['read_channels'],
        mode: 'hidden',
      },
      {
        path: '/admin/prompt-protection-rules',
        requiredScopes: ['read_channels'],
        mode: 'hidden',
      },
      {
        path: '/admin/data-storages',
        requiredScopes: ['read_data_storages'],
        mode: 'hidden',
      },
      {
        path: '/admin/users',
        requiredScopes: ['read_users'],
        mode: 'hidden',
      },
      {
        path: '/admin/roles',
        requiredScopes: ['read_roles'],
        mode: 'hidden',
      },
      {
        path: '/admin/oauth-applications',
        requiredScopes: ['read_settings'],
        mode: 'hidden',
      },
      {
        path: '/api-keys',
        requiredScopes: ['read_api_keys'],
        mode: 'hidden',
      },
      {
        path: '/admin/system',
        requiredScopes: ['read_settings'],
        mode: 'hidden',
      },
      {
        path: '/admin/runtime',
        requiredScopes: ['read_settings'],
        mode: 'hidden',
      },
      {
        path: '/analytics',
        requiredScopes: ['read_dashboard'],
        scopeLevel: 'system',
        mode: 'hidden',
      },
    ],
  },
  {
    title: 'Personal',
    scopeLevel: 'any', // Personal 路由组可以通过 system-level 或 project-level 权限访问
    routes: [
      {
        path: '/',
        // Personal dashboard - accessible to all users
      },
      {
        path: '/my-channels',
        // Personal channels - accessible to all users
      },
      {
        path: '/my-models',
        // Personal models - accessible to all users
      },
      {
        path: '/shared',
        // Shared with me - accessible to all users
      },
      {
        path: '/project/api-keys',
        requiredScopes: ['read_api_keys'],
        mode: 'hidden',
      },
      {
        path: '/project/prompts',
        requiredScopes: ['read_prompts'],
        mode: 'hidden',
      },
      {
        path: '/project/requests',
        requiredScopes: ['read_requests'],
        mode: 'hidden',
      },
      {
        path: '/project/usage-logs',
        requiredScopes: ['read_requests'],
        mode: 'hidden',
      },
      {
        path: '/project/usage-stats',
        requiredScopes: ['read_requests'],
        mode: 'hidden',
        requireProjectOwner: true,
      },
      {
        path: '/project/traces',
        requiredScopes: ['read_requests'],
        mode: 'hidden',
      },
      {
        path: '/project/traces/$traceId',
        requiredScopes: ['read_requests'],
        mode: 'hidden',
      },
      {
        path: '/project/threads',
        requiredScopes: ['read_requests'],
        mode: 'hidden',
      },
      {
        path: '/project/threads/$threadId',
        requiredScopes: ['read_requests'],
        mode: 'hidden',
      },
    ],
  },
  {
    title: 'Settings',
    routes: [
      {
        path: '/settings',
        // Profile 设置所有用户都可以访问
      },
      {
        path: '/settings/profile',
        // Profile 设置所有用户都可以访问
      },
      {
        path: '/settings/appearance',
        // Appearance 设置所有用户都可以访问
      },
      {
        path: '/settings/notifications',
        // Notifications 设置所有用户都可以访问
      },
    ],
  },
];

// 获取路由配置的辅助函数
export function getRouteConfig(path: string): RouteConfig | undefined {
  for (const group of routeConfigs) {
    for (const route of group.routes) {
      if (route.path === path) {
        return route;
      }
      if (route.children) {
        const childConfig = route.children.find((child) => child.path === path);
        if (childConfig) return childConfig;
      }
    }
  }
  return undefined;
}

// 检查用户是否有访问路由的权限
export function hasRouteAccess(userScopes: string[], routeConfig: RouteConfig): boolean {
  if (!routeConfig.requiredScopes || routeConfig.requiredScopes.length === 0) {
    return true;
  }

  // 如果用户有通配符权限，则拥有所有权限
  if (userScopes.includes('*')) {
    return true;
  }

  // 检查用户是否拥有所需的任一权限
  return routeConfig.requiredScopes.some((scope) => userScopes.includes(scope));
}

// 检查用户是否有访问路由组的权限
export function hasGroupAccess(userScopes: string[], group: RouteGroup): boolean {
  return group.routes.some((route) => hasRouteAccess(userScopes, route));
}

// 根据路径查找路由配置及其所属组的 scopeLevel
export function getRouteConfigByPathWithGroup(path: string): {
  routeConfig?: RouteConfig;
  groupScopeLevel?: ScopeLevel;
} {
  for (const group of routeConfigs) {
    for (const route of group.routes) {
      if (route.path === path) {
        return { routeConfig: route, groupScopeLevel: group.scopeLevel };
      }
      if (route.children) {
        const childConfig = route.children.find((child) => child.path === path);
        if (childConfig) {
          return { routeConfig: childConfig, groupScopeLevel: group.scopeLevel };
        }
      }
    }
  }
  return {};
}

// Detail paths without an explicit config (e.g. /admin/requests/123) inherit
// the config of their parent page by walking up one segment at a time.
export function resolveRouteConfigForPath(pathname: string): {
  routeConfig?: RouteConfig;
  groupScopeLevel?: ScopeLevel;
} {
  let path = pathname;
  while (path) {
    const found = getRouteConfigByPathWithGroup(path);
    if (found.routeConfig) {
      return found;
    }
    const lastSlash = path.lastIndexOf('/');
    if (lastSlash <= 0) {
      break;
    }
    path = path.slice(0, lastSlash);
  }
  return {};
}

export interface PathAccessUser {
  isOwner?: boolean;
  isAdmin?: boolean;
  scopes?: string[];
  projects?: { projectID: string; isOwner: boolean; scopes?: string[]; effectiveScopes?: string[] }[];
}

// Non-hook variant of useRoutePermissions.hasRouteAccess for use in route
// beforeLoad guards; reads plain user data so it can run outside React.
// Paths with no config at all stay owner-only (safe default).
export function hasPathAccess(user: PathAccessUser | null | undefined, pathname: string, selectedProjectId?: string | null): boolean {
  const { routeConfig, groupScopeLevel } = resolveRouteConfigForPath(pathname);

  if (!routeConfig) {
    return user?.isOwner === true;
  }

  const project = selectedProjectId ? user?.projects?.find((p) => p.projectID === selectedProjectId) : undefined;
  const isProjectOwner = user?.isOwner === true || user?.isAdmin === true || project?.isOwner === true;

  if (routeConfig.requireProjectOwner && !isProjectOwner) {
    return false;
  }

  if (!routeConfig.requiredScopes || routeConfig.requiredScopes.length === 0) {
    return true;
  }

  const scopeLevel = routeConfig.scopeLevel || groupScopeLevel || 'any';

  // system 级路由（管理组）按身份硬门槛：仅 owner / admin 可进入，
  // 与 scope 解耦——普通用户即使持有对应 scope 也会被守卫拦下。
  if (scopeLevel === 'system' && user?.isOwner !== true && user?.isAdmin !== true) {
    return false;
  }

  if (user?.isOwner || user?.isAdmin) {
    return true;
  }

  if (isProjectOwner && scopeLevel !== 'system') {
    return true;
  }

  const systemScopes = user?.scopes ?? [];
  const projectScopes = project?.effectiveScopes || project?.scopes || [];
  const scopesToCheck =
    scopeLevel === 'system' ? systemScopes : scopeLevel === 'project' ? projectScopes : [...systemScopes, ...projectScopes];

  if (scopesToCheck.includes('*')) {
    return true;
  }

  return routeConfig.requiredScopes.some((scope) => scopesToCheck.includes(scope));
}
