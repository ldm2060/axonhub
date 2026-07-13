export interface RequestNavigationPageInfo {
  hasPreviousPage: boolean;
  hasNextPage: boolean;
  startCursor?: string | null;
  endCursor?: string | null;
}

export interface NavigationPage<T, P extends RequestNavigationPageInfo> {
  items: T[];
  pageInfo: P;
}

export interface NavigationState<T, P extends RequestNavigationPageInfo> {
  pages: NavigationPage<T, P>[];
  currentIndex: number;
}

export type NavigationDirection = 'older' | 'newer';

export function flattenNavigationPages<T, P extends RequestNavigationPageInfo>(pages: NavigationPage<T, P>[]): T[] {
  return pages.flatMap(({ items }) => items);
}

export function createNavigationState<T, P extends RequestNavigationPageInfo>(
  page: NavigationPage<T, P>,
  currentIndex: number
): NavigationState<T, P> {
  return { pages: page.items.length > 0 ? [page] : [], currentIndex };
}

export function mergeNavigationPage<T extends { id: string }, P extends RequestNavigationPageInfo>(
  state: NavigationState<T, P>,
  incomingPage: NavigationPage<T, P>,
  direction: NavigationDirection,
  maxPages: number
): NavigationState<T, P> {
  const targetId = direction === 'older' ? incomingPage.items[0]?.id : incomingPage.items.at(-1)?.id;
  const orderedPages = direction === 'older' ? [...state.pages, incomingPage] : [incomingPage, ...state.pages];
  const seen = new Set<string>();
  const deduplicatedPages = orderedPages
    .map((page) => ({
      ...page,
      items: page.items.filter(({ id }) => {
        if (seen.has(id)) return false;
        seen.add(id);
        return true;
      }),
    }))
    .filter(({ items }) => items.length > 0);
  const evictedNewerPage = direction === 'older' && deduplicatedPages.length > maxPages;
  const evictedOlderPage = direction === 'newer' && deduplicatedPages.length > maxPages;
  const pages = direction === 'older' ? deduplicatedPages.slice(-maxPages) : deduplicatedPages.slice(0, maxPages);

  if (evictedNewerPage && pages[0]) {
    pages[0] = {
      ...pages[0],
      pageInfo: { ...pages[0].pageInfo, hasPreviousPage: true },
    };
  }
  if (evictedOlderPage && pages.at(-1)) {
    const lastIndex = pages.length - 1;
    pages[lastIndex] = {
      ...pages[lastIndex],
      pageInfo: { ...pages[lastIndex].pageInfo, hasNextPage: true },
    };
  }

  const items = flattenNavigationPages(pages);
  const targetIndex = targetId ? items.findIndex(({ id }) => id === targetId) : -1;

  return { pages, currentIndex: targetIndex >= 0 ? targetIndex : state.currentIndex };
}
