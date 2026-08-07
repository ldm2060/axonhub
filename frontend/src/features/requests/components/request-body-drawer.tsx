'use client';

import { useState, useCallback, useEffect, useRef } from 'react';
import { useNavigate } from '@tanstack/react-router';
import { ChevronLeft, ChevronRight, ExternalLink, FileText, ChevronsDownUp, ChevronsUpDown, Copy, Terminal } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { useSelectedProjectId } from '@/stores/projectStore';
import { extractNumberID } from '@/lib/utils';
import { usePaginationSearch } from '@/hooks/use-pagination-search';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Sheet, SheetContent, SheetHeader, SheetTitle } from '@/components/ui/sheet';
import { Skeleton } from '@/components/ui/skeleton';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { JsonViewer } from '@/components/json-tree-view';
import { useRequestPermissions } from '../../../hooks/useRequestPermissions';
import { useRequest, fetchAdjacentRequestPage } from '../data';
import { Request, RequestConnection } from '../data/schema';
import { generateRequestCurl } from '../utils/curl-generator';
import { CurlPreviewDialog } from './curl-preview-dialog';
import { getStatusColor } from './help';
import { createNavigationState, flattenNavigationPages, mergeNavigationPage, type NavigationState } from './request-navigation-state';

interface RequestBodyDrawerProps {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  /** ID of the request that was clicked. */
  initialRequestId: string | null;
  /** Position of initialRequestId within initialRequests. */
  initialIndex: number;
  /** Current page's request list (DESC order). */
  initialRequests: Request[];
  pageInfo?: RequestConnection['pageInfo'];
  /** Optional server-side filter currently applied to the table. */
  queryWhere?: Record<string, any>;
  projectId?: string | null;
  includeAdminFields?: boolean;
  onViewDetail?: (requestId: string) => void;
}

interface RequestBodyDrawerContentProps {
  currentRequestId: string;
  projectId?: string | null;
  includeAdminFields: boolean;
}

type RequestPageInfo = RequestConnection['pageInfo'];

const OPEN_ANIMATION_DELAY_MS = 520;
const MAX_NAVIGATION_PAGES = 3;
const EMPTY_PAGE_INFO: RequestPageInfo = {
  hasNextPage: false,
  hasPreviousPage: false,
};

function RequestBodyDrawerContent({ currentRequestId, projectId, includeAdminFields }: RequestBodyDrawerContentProps) {
  const { t } = useTranslation();
  const {
    data: request,
    isLoading,
    isFetching,
  } = useRequest(currentRequestId, {
    projectId,
    enabled: true,
    includeAdminFields,
    gcTime: 0,
    queryScope: 'quick-view',
  });
  const displayedRequestRef = useRef<Request | null>(null);
  const [globalExpanded, setGlobalExpanded] = useState(false);
  const [activeTab, setActiveTab] = useState('request');
  const [showCurlPreview, setShowCurlPreview] = useState(false);
  const [curlCommand, setCurlCommand] = useState('');

  if (request) displayedRequestRef.current = request;
  const displayedRequest = displayedRequestRef.current;

  const copyBody = useCallback(
    (data: any) => {
      try {
        navigator.clipboard.writeText(JSON.stringify(data, null, 2));
      } catch {
        navigator.clipboard.writeText(String(data));
      }
      toast.success(t('requests.actions.copy'));
    },
    [t]
  );

  const handleCurlPreview = useCallback(() => {
    if (!displayedRequest) return;
    const curl = generateRequestCurl(displayedRequest.requestHeaders, displayedRequest.requestBody, displayedRequest.format as any);
    setCurlCommand(curl);
    setShowCurlPreview(true);
  }, [displayedRequest]);

  if (!displayedRequest && isLoading) {
    return (
      <div className='space-y-4 p-6'>
        <Skeleton className='h-8 w-full' />
        <Skeleton className='h-64 w-full' />
        <Skeleton className='h-32 w-full' />
      </div>
    );
  }

  if (!displayedRequest) return null;

  return (
    <>
      <div className='relative flex min-h-0 flex-1 flex-col'>
        {isFetching && <div className='bg-primary/40 absolute inset-x-0 top-0 z-10 h-0.5 animate-pulse' />}
        <Tabs value={activeTab} onValueChange={setActiveTab} className='flex h-full flex-col'>
          <div className='mx-6 mt-4 flex flex-shrink-0 items-center gap-2'>
            <TabsList className='grid flex-1 grid-cols-2'>
              <TabsTrigger value='request'>{t('requests.detail.tabs.request')}</TabsTrigger>
              <TabsTrigger value='response'>{t('requests.detail.tabs.response')}</TabsTrigger>
            </TabsList>
            <Button
              variant='outline'
              size='icon'
              className='h-9 w-9 flex-shrink-0'
              onClick={() => setGlobalExpanded((value) => !value)}
              title={globalExpanded ? t('requests.drawer.collapseAll') : t('requests.drawer.expandAll')}
            >
              {globalExpanded ? <ChevronsDownUp className='h-4 w-4' /> : <ChevronsUpDown className='h-4 w-4' />}
            </Button>
            <Button
              variant='outline'
              size='icon'
              className='h-9 w-9 flex-shrink-0'
              onClick={() => copyBody(activeTab === 'request' ? displayedRequest.requestBody : displayedRequest.responseBody)}
              title={t('requests.actions.copy')}
            >
              <Copy className='h-4 w-4' />
            </Button>
            {activeTab === 'request' && (
              <Button
                variant='outline'
                size='icon'
                className='h-9 w-9 flex-shrink-0'
                onClick={handleCurlPreview}
                title={t('requests.actions.copyCurl')}
              >
                <Terminal className='h-4 w-4' />
              </Button>
            )}
          </div>

          <TabsContent value='request' className='m-0 min-h-0 flex-1 px-6 pt-4 pb-6'>
            <ScrollArea className='bg-muted/20 h-full w-full rounded-lg border p-4'>
              {displayedRequest.requestBody ? (
                <JsonViewer
                  key={`req-${currentRequestId}`}
                  data={displayedRequest.requestBody}
                  rootName=''
                  defaultExpanded={true}
                  expandDepth='all'
                  hideArrayIndices={true}
                  globalStringExpanded={globalExpanded}
                  className='text-sm'
                />
              ) : (
                <div className='flex h-32 items-center justify-center'>
                  <p className='text-muted-foreground text-sm'>{t('requests.drawer.noRequestBody')}</p>
                </div>
              )}
            </ScrollArea>
          </TabsContent>

          <TabsContent value='response' className='m-0 min-h-0 flex-1 px-6 pt-4 pb-6'>
            <ScrollArea className='bg-muted/20 h-full w-full rounded-lg border p-4'>
              {displayedRequest.responseBody ? (
                <JsonViewer
                  key={`res-${currentRequestId}`}
                  data={displayedRequest.responseBody}
                  rootName=''
                  defaultExpanded={true}
                  expandDepth='all'
                  hideArrayIndices={true}
                  globalStringExpanded={globalExpanded}
                  className='text-sm'
                />
              ) : (
                <div className='flex h-32 items-center justify-center'>
                  <p className='text-muted-foreground text-sm'>{t('requests.detail.noResponse')}</p>
                </div>
              )}
            </ScrollArea>
          </TabsContent>
        </Tabs>
      </div>
      <CurlPreviewDialog open={showCurlPreview} onOpenChange={setShowCurlPreview} curlCommand={curlCommand} />
    </>
  );
}

export function RequestBodyDrawer({
  open,
  onOpenChange,
  initialRequestId,
  initialIndex,
  initialRequests,
  pageInfo: initialPageInfo,
  queryWhere,
  projectId,
  includeAdminFields = false,
  onViewDetail,
}: RequestBodyDrawerProps) {
  const { t } = useTranslation();
  const navigate = useNavigate();
  const { navigateWithSearch } = usePaginationSearch({ defaultPageSize: 20 });
  const permissions = useRequestPermissions({ systemOnly: projectId === null });
  const selectedProjectId = useSelectedProjectId();
  const effectiveProjectId = projectId !== undefined ? projectId : selectedProjectId;
  const [navigation, setNavigation] = useState<NavigationState<Request, RequestPageInfo>>({
    pages: [],
    currentIndex: 0,
  });
  const [isLoadingMore, setIsLoadingMore] = useState(false);
  const prevOpenRef = useRef(false);
  const navigationGenerationRef = useRef(0);

  const isOpeningBeforeStateSync = open && !prevOpenRef.current;
  const visibleNavigation = isOpeningBeforeStateSync
    ? createNavigationState({ items: initialRequests, pageInfo: initialPageInfo ?? EMPTY_PAGE_INFO }, initialIndex)
    : navigation;
  const visibleRequests = flattenNavigationPages(visibleNavigation.pages);
  const currentIndex = visibleNavigation.currentIndex;
  const currentRequestId = visibleRequests[currentIndex]?.id ?? initialRequestId;
  const listRequest = visibleRequests[currentIndex];
  const firstPageInfo = visibleNavigation.pages[0]?.pageInfo;
  const lastPageInfo = visibleNavigation.pages.at(-1)?.pageInfo;

  useEffect(() => {
    const justOpened = open && !prevOpenRef.current;
    prevOpenRef.current = open;

    if (justOpened) {
      navigationGenerationRef.current += 1;
      setNavigation(createNavigationState({ items: initialRequests, pageInfo: initialPageInfo ?? EMPTY_PAGE_INFO }, initialIndex));
      return;
    }

    if (!open) {
      navigationGenerationRef.current += 1;
      setNavigation({ pages: [], currentIndex: 0 });
      setIsLoadingMore(false);
    }
  }, [open, initialRequests, initialPageInfo, initialIndex]);

  const [canRenderBody, setCanRenderBody] = useState(false);
  useEffect(() => {
    if (!open) {
      setCanRenderBody(false);
      return;
    }

    setCanRenderBody(false);
    const timeoutId = setTimeout(() => setCanRenderBody(true), OPEN_ANIMATION_DELAY_MS);

    return () => {
      clearTimeout(timeoutId);
    };
  }, [open, initialRequestId]);

  const canGoPrev = currentIndex < visibleRequests.length - 1 || !!lastPageInfo?.hasNextPage;
  const canGoNext = currentIndex > 0 || !!firstPageInfo?.hasPreviousPage;

  const handlePrev = useCallback(async () => {
    if (currentIndex < visibleRequests.length - 1) {
      setNavigation((current) => ({ ...current, currentIndex: current.currentIndex + 1 }));
      return;
    }
    if (!lastPageInfo?.hasNextPage || !lastPageInfo.endCursor || isLoadingMore) return;

    const generation = navigationGenerationRef.current;
    setIsLoadingMore(true);
    try {
      const result = await fetchAdjacentRequestPage({
        cursor: lastPageInfo.endCursor,
        direction: 'older',
        pageSize: initialRequests.length || 20,
        where: queryWhere,
        permissions,
        projectId: effectiveProjectId,
        includeAdminFields,
      });
      if (generation !== navigationGenerationRef.current) return;

      setNavigation((current) =>
        mergeNavigationPage(current, { items: result.requests, pageInfo: result.pageInfo }, 'older', MAX_NAVIGATION_PAGES)
      );
    } finally {
      if (generation === navigationGenerationRef.current) setIsLoadingMore(false);
    }
  }, [
    currentIndex,
    visibleRequests.length,
    lastPageInfo,
    isLoadingMore,
    initialRequests.length,
    queryWhere,
    permissions,
    effectiveProjectId,
    includeAdminFields,
  ]);

  const handleNext = useCallback(async () => {
    if (currentIndex > 0) {
      setNavigation((current) => ({ ...current, currentIndex: current.currentIndex - 1 }));
      return;
    }
    if (!firstPageInfo?.hasPreviousPage || !firstPageInfo.startCursor || isLoadingMore) return;

    const generation = navigationGenerationRef.current;
    setIsLoadingMore(true);
    try {
      const result = await fetchAdjacentRequestPage({
        cursor: firstPageInfo.startCursor,
        direction: 'newer',
        pageSize: initialRequests.length || 20,
        where: queryWhere,
        permissions,
        projectId: effectiveProjectId,
        includeAdminFields,
      });
      if (generation !== navigationGenerationRef.current) return;

      setNavigation((current) =>
        mergeNavigationPage(current, { items: result.requests, pageInfo: result.pageInfo }, 'newer', MAX_NAVIGATION_PAGES)
      );
    } finally {
      if (generation === navigationGenerationRef.current) setIsLoadingMore(false);
    }
  }, [currentIndex, firstPageInfo, isLoadingMore, initialRequests.length, queryWhere, permissions, effectiveProjectId, includeAdminFields]);

  const handleViewDetail = useCallback(() => {
    if (!currentRequestId) return;

    const numericId = extractNumberID(currentRequestId) || currentRequestId;
    if (onViewDetail) {
      onViewDetail(currentRequestId);
    } else if (effectiveProjectId) {
      navigateWithSearch({
        to: '/project/requests/$requestId',
        params: { requestId: numericId },
      });
    } else {
      navigate({
        to: '/requests/$requestId',
        params: { requestId: numericId },
      });
    }
    onOpenChange(false);
  }, [currentRequestId, onViewDetail, effectiveProjectId, navigateWithSearch, navigate, onOpenChange]);

  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side='right' className='flex w-[min(100vw,clamp(500px,50vw,800px))] max-w-none flex-col gap-0 p-0 sm:max-w-none'>
        <SheetHeader className='flex-shrink-0 border-b px-6 py-4'>
          <div className='flex items-center justify-between pr-6'>
            <SheetTitle className='flex items-center gap-2 text-base'>
              <FileText className='h-4 w-4' />
              {listRequest ? (
                <>
                  <span className='font-mono'>#{extractNumberID(listRequest.id)}</span>
                  <Badge className={getStatusColor(listRequest.status)} variant='secondary'>
                    {t(`requests.status.${listRequest.status}`)}
                  </Badge>
                </>
              ) : null}
            </SheetTitle>

            <div className='flex items-center gap-1.5'>
              <Button
                variant='outline'
                size='icon'
                className='h-7 w-7'
                onClick={handlePrev}
                disabled={!canGoPrev || isLoadingMore}
                title={t('requests.drawer.previous')}
              >
                <ChevronLeft className='h-4 w-4' />
              </Button>
              <Button
                variant='outline'
                size='icon'
                className='h-7 w-7'
                onClick={handleNext}
                disabled={!canGoNext || isLoadingMore}
                title={t('requests.drawer.next')}
              >
                <ChevronRight className='h-4 w-4' />
              </Button>
              <Button variant='outline' size='sm' onClick={handleViewDetail} className='ml-1 h-7 text-xs'>
                <ExternalLink className='mr-1 h-3.5 w-3.5' />
                {t('requests.drawer.viewDetail')}
              </Button>
            </div>
          </div>
        </SheetHeader>

        <div className='flex min-h-0 flex-1 flex-col'>
          {open && canRenderBody && currentRequestId ? (
            <RequestBodyDrawerContent
              currentRequestId={currentRequestId}
              projectId={effectiveProjectId}
              includeAdminFields={includeAdminFields}
            />
          ) : open ? (
            <div className='space-y-4 p-6'>
              <Skeleton className='h-8 w-full' />
              <Skeleton className='h-64 w-full' />
              <Skeleton className='h-32 w-full' />
            </div>
          ) : null}
        </div>
      </SheetContent>
    </Sheet>
  );
}
