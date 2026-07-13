import { useCallback, useEffect, useMemo, useState } from 'react';
import { DashboardIcon } from '@radix-ui/react-icons';
import { format } from 'date-fns';
import { enUS, zhCN } from 'date-fns/locale';
import { ChevronDown, ChevronRight, Copy, Database, Download, FileText, Key, Layers, Terminal } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { extractNumberID } from '@/lib/utils';
import { getTokenFromStorage } from '@/stores/authStore';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { JsonViewer } from '@/components/json-tree-view';
import { useGeneralSettings } from '@/features/system/data/system';
import {
  type RequestMetadata,
  useRequestContent,
  useRequestExecutions,
} from '../data';
import { generateRequestCurl } from '../utils/curl-generator';
import { parseResponse } from '../utils/response-parser';
import { ChunksDialog } from './chunks-dialog';
import { CurlPreviewDialog } from './curl-preview-dialog';
import { getStatusColor } from './help';
import { nextExpandedExecution, type RequestDetailTab } from './request-content-state';
import { RequestExecutionContentPanel } from './request-execution-content';
import { ResponseFlow } from './response-flow';

interface RequestDetailContentProps {
  request?: RequestMetadata | null;
  requestId: string;
  projectId?: string | null;
  includeAdminFields?: boolean;
  activeTab: RequestDetailTab;
  onActiveTabChange: (tab: RequestDetailTab) => void;
  previewRequest?: RequestMetadata | null;
  previewChunks?: any[];
  previewVersion?: number;
  isPreviewStreaming?: boolean;
}

interface ContentPanelProps {
  request: RequestMetadata;
  requestId: string;
  projectId?: string | null;
  includeAdminFields?: boolean;
}

function formatJson(data: any) {
  if (!data) return '';
  try {
    return JSON.stringify(data, null, 2);
  } catch {
    return String(data);
  }
}

function copyToClipboard(text: string, successMessage: string) {
  void navigator.clipboard.writeText(text);
  toast.success(successMessage);
}

function downloadFile(content: string, filename: string, successMessage: string) {
  const blob = new Blob([content], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  document.body.appendChild(anchor);
  anchor.click();
  document.body.removeChild(anchor);
  URL.revokeObjectURL(url);
  toast.success(successMessage);
}

function ContentLoading() {
  const { t } = useTranslation();
  return (
    <div className='flex min-h-80 items-center justify-center'>
      <div className='space-y-4 text-center'>
        <div className='border-primary mx-auto h-10 w-10 animate-spin rounded-full border-b-2' />
        <p className='text-muted-foreground'>{t('common.loading')}</p>
      </div>
    </div>
  );
}

function ContentError({ retry }: { retry: () => void }) {
  const { t } = useTranslation();
  return (
    <div className='flex min-h-80 items-center justify-center'>
      <div className='space-y-4 text-center'>
        <FileText className='text-muted-foreground mx-auto h-12 w-12' />
        <p className='text-muted-foreground'>{t('requests.detail.loadFailed')}</p>
        <Button variant='outline' onClick={retry}>{t('requests.detail.retry')}</Button>
      </div>
    </div>
  );
}

function RequestContentPanel({ request, requestId, projectId, includeAdminFields }: ContentPanelProps) {
  const { t } = useTranslation();
  const { data, isLoading, isError, refetch } = useRequestContent(requestId, {
    kind: 'request',
    enabled: true,
    projectId,
    includeAdminFields,
  });
  const [curlCommand, setCurlCommand] = useState('');
  const [showCurlPreview, setShowCurlPreview] = useState(false);

  if (isLoading) return <ContentLoading />;
  if (isError) return <ContentError retry={() => void refetch()} />;

  const requestHeaders = data?.requestHeaders;
  const requestBody = data?.requestBody;

  return (
    <div className='space-y-6'>
      <div className='flex justify-end'>
        <Button
          variant='outline'
          size='sm'
          onClick={() => {
            setCurlCommand(generateRequestCurl(requestHeaders, requestBody, request.format as any));
            setShowCurlPreview(true);
          }}
          disabled={!requestHeaders && !requestBody}
        >
          <Terminal className='mr-2 h-4 w-4' />
          {t('requests.actions.copyCurl')}
        </Button>
      </div>

      {requestHeaders && (
        <section className='space-y-4'>
          <div className='flex items-center justify-between gap-4'>
            <h4 className='flex items-center gap-2 font-semibold'><FileText className='text-primary h-4 w-4' />{t('requests.columns.requestHeaders')}</h4>
            <div className='flex gap-2'>
              <Button variant='outline' size='sm' onClick={() => copyToClipboard(formatJson(requestHeaders), t('requests.actions.copy'))}><Copy className='mr-2 h-4 w-4' />{t('requests.dialogs.jsonViewer.copy')}</Button>
              <Button variant='outline' size='sm' onClick={() => downloadFile(formatJson(requestHeaders), `request-headers-${request.id}.json`, t('requests.actions.download'))}><Download className='mr-2 h-4 w-4' />{t('requests.dialogs.jsonViewer.download')}</Button>
            </div>
          </div>
          <div className='bg-muted/20 h-[300px] overflow-auto rounded-lg border p-4'>
            <JsonViewer data={requestHeaders} rootName='' defaultExpanded expandDepth='all' hideArrayIndices className='text-sm' />
          </div>
        </section>
      )}

      <section className='space-y-4'>
        <div className='flex items-center justify-between gap-4'>
          <h4 className='flex items-center gap-2 font-semibold'><FileText className='text-primary h-4 w-4' />{t('requests.columns.requestBody')}</h4>
          <div className='flex gap-2'>
            <Button variant='outline' size='sm' disabled={!requestBody} onClick={() => copyToClipboard(formatJson(requestBody), t('requests.actions.copy'))}><Copy className='mr-2 h-4 w-4' />{t('requests.dialogs.jsonViewer.copy')}</Button>
            <Button variant='outline' size='sm' disabled={!requestBody} onClick={() => downloadFile(formatJson(requestBody), `request-body-${request.id}.json`, t('requests.actions.download'))}><Download className='mr-2 h-4 w-4' />{t('requests.dialogs.jsonViewer.download')}</Button>
          </div>
        </div>
        <div className='bg-muted/20 h-[500px] overflow-auto rounded-lg border p-4'>
          {requestBody
            ? <JsonViewer data={requestBody} rootName='' defaultExpanded expandDepth='all' hideArrayIndices className='text-sm' />
            : <div className='text-muted-foreground flex h-full items-center justify-center'>{t('requests.drawer.noRequestBody')}</div>}
        </div>
      </section>

      <CurlPreviewDialog
        open={showCurlPreview}
        onOpenChange={(open) => {
          setShowCurlPreview(open);
          if (!open) setCurlCommand('');
        }}
        curlCommand={curlCommand}
      />
    </div>
  );
}

interface ResponseContentPanelProps extends ContentPanelProps {
  previewRequest?: RequestMetadata | null;
  previewChunks?: any[];
  previewVersion: number;
  isPreviewStreaming: boolean;
}

function ResponseContentPanel({
  request,
  requestId,
  projectId,
  includeAdminFields,
  previewRequest,
  previewChunks,
  previewVersion,
  isPreviewStreaming,
}: ResponseContentPanelProps) {
  const { t } = useTranslation();
  const shouldLoadPersistedResponse = !previewRequest && !(request.status === 'processing' && request.stream === true);
  const { data, isLoading, isError, refetch } = useRequestContent(requestId, {
    kind: 'response',
    enabled: shouldLoadPersistedResponse,
    projectId,
    includeAdminFields,
  });
  const [responseView, setResponseView] = useState<'preview' | 'json'>('preview');
  const [showChunks, setShowChunks] = useState(false);
  const [isDownloading, setIsDownloading] = useState(false);
  const [audioObjectUrl, setAudioObjectUrl] = useState<string | null>(null);
  const [audioLoadFailed, setAudioLoadFailed] = useState(false);

  const responseBody = previewRequest ? undefined : data?.responseBody;
  const responseChunks = previewRequest ? previewChunks : data?.responseChunks;
  const parsedResponse = useMemo(
    () => parseResponse(responseBody, responseChunks),
    [previewVersion, responseBody, responseChunks]
  );
  const isLive = isPreviewStreaming || (request.status === 'processing' && request.stream === true);
  const isSpeechRequest = request.format === 'openai/audio_speech';
  const isVideoRequest = request.format === 'openai/video' || request.format === 'seedance/video';
  const hasStoredContent = !!(request.contentSaved && request.contentStorageKey);

  const fetchStoredContent = useCallback(async () => {
    if (!hasStoredContent || !projectId) return null;
    const numericId = extractNumberID(request.id);
    const token = getTokenFromStorage();
    if (!numericId || !token) return null;
    const response = await fetch(`/admin/requests/${encodeURIComponent(numericId)}/content`, {
      headers: { Authorization: `Bearer ${token}`, 'X-Project-ID': projectId },
    });
    if (!response.ok) throw new Error(`HTTP ${response.status}`);
    const disposition = response.headers.get('Content-Disposition') || '';
    const filename = disposition.match(/filename="?([^";]+)"?/i)?.[1] ?? '';
    return { blob: await response.blob(), filename };
  }, [hasStoredContent, projectId, request.id]);

  useEffect(() => {
    if (!isSpeechRequest || !hasStoredContent) return;
    let cancelled = false;
    let objectUrl: string | null = null;
    void fetchStoredContent()
      .then((result) => {
        if (cancelled || !result) return;
        objectUrl = URL.createObjectURL(result.blob);
        setAudioObjectUrl(objectUrl);
      })
      .catch(() => {
        if (!cancelled) setAudioLoadFailed(true);
      });
    return () => {
      cancelled = true;
      if (objectUrl) URL.revokeObjectURL(objectUrl);
      setAudioObjectUrl(null);
    };
  }, [fetchStoredContent, hasStoredContent, isSpeechRequest]);

  if (shouldLoadPersistedResponse && isLoading) return <ContentLoading />;
  if (shouldLoadPersistedResponse && isError) return <ContentError retry={() => void refetch()} />;

  const responseText = [parsedResponse.reasoning, parsedResponse.content]
    .filter(Boolean)
    .concat(parsedResponse.toolCalls.map((toolCall) => `Tool Call: ${toolCall.function?.name}\nArguments: ${toolCall.function?.arguments}`))
    .join('\n\n')
    .trim();

  const downloadStoredContent = async () => {
    try {
      setIsDownloading(true);
      const result = await fetchStoredContent();
      if (!result) return;
      const numericId = extractNumberID(request.id);
      const fallback = isVideoRequest ? `video-${numericId}.mp4` : `audio-${numericId}.mp3`;
      const url = URL.createObjectURL(result.blob);
      const anchor = document.createElement('a');
      anchor.href = url;
      anchor.download = result.filename || fallback;
      anchor.click();
      URL.revokeObjectURL(url);
      toast.success(t('requests.actions.download'));
    } catch {
      toast.error(t('common.errors.operationFailed', { operation: t('requests.actions.download') }));
    } finally {
      setIsDownloading(false);
    }
  };

  return (
    <div className='space-y-6'>
      <Tabs value={responseView} onValueChange={(value) => setResponseView(value as 'preview' | 'json')}>
        <div className='flex flex-wrap items-center justify-between gap-4'>
          <TabsList className='grid w-full grid-cols-2 sm:w-[300px]'>
            <TabsTrigger value='preview'>{t('requests.detail.tabs.preview')}</TabsTrigger>
            <TabsTrigger value='json'>{t('requests.detail.tabs.json')}</TabsTrigger>
          </TabsList>
          <div className='flex flex-wrap gap-2'>
            {(isVideoRequest || isSpeechRequest) && hasStoredContent && (
              <Button variant='outline' size='sm' disabled={isDownloading} onClick={() => void downloadStoredContent()}><Download className='mr-2 h-4 w-4' />{isVideoRequest ? t('requests.actions.downloadVideo') : t('requests.actions.downloadAudio')}</Button>
            )}
            <Button variant='outline' size='sm' disabled={!responseChunks?.length} onClick={() => setShowChunks(true)}><Layers className='mr-2 h-4 w-4' />{isLive ? t('requests.actions.preview') : t('requests.columns.responseChunks')}</Button>
            <Button variant='outline' size='sm' disabled={responseView === 'preview' ? !responseText : !responseBody} onClick={() => copyToClipboard(responseView === 'preview' ? responseText : formatJson(responseBody), t('requests.actions.copy'))}><Copy className='mr-2 h-4 w-4' />{t('requests.dialogs.jsonViewer.copy')}</Button>
            <Button variant='outline' size='sm' disabled={!responseBody} onClick={() => downloadFile(formatJson(responseBody), `response-body-${request.id}.json`, t('requests.actions.download'))}><Download className='mr-2 h-4 w-4' />{t('requests.dialogs.jsonViewer.download')}</Button>
          </div>
        </div>

        <TabsContent value='preview' className='mt-6'>
          {isSpeechRequest ? (
            <div className='bg-muted/20 flex min-h-52 items-center justify-center rounded-lg border p-6'>
              {audioObjectUrl
                ? <audio controls src={audioObjectUrl} className='w-full max-w-xl' />
                : <p className='text-muted-foreground'>{audioLoadFailed ? t('requests.detail.audioLoadFailed') : t('common.loading')}</p>}
            </div>
          ) : responseText || isLive ? (
            <ResponseFlow chunks={responseChunks} version={previewVersion} body={responseBody} isLive={isLive} reasoningDurationMs={request.metricsReasoningDurationMs} />
          ) : (
            <div className='bg-muted/20 text-muted-foreground flex h-96 items-center justify-center rounded-lg border'>{t('requests.detail.noResponse')}</div>
          )}
        </TabsContent>
        <TabsContent value='json' className='mt-6'>
          <div className='bg-muted/20 h-[500px] overflow-auto rounded-lg border p-4'>
            {responseBody
              ? <JsonViewer data={responseBody} rootName='' defaultExpanded expandDepth='all' hideArrayIndices className='text-sm' />
              : <div className='text-muted-foreground flex h-full items-center justify-center'>{t('requests.detail.noResponse')}</div>}
          </div>
        </TabsContent>
      </Tabs>

      <ChunksDialog
        open={showChunks}
        onOpenChange={setShowChunks}
        chunks={responseChunks ?? []}
        isLive={isLive}
        title={t('requests.dialogs.jsonViewer.responseChunks')}
      />
    </div>
  );
}

function OverviewPanel({ request }: { request: RequestMetadata }) {
  const { t, i18n } = useTranslation();
  const { data: settings } = useGeneralSettings();
  const usage = request.usageLogs?.edges?.[0]?.node;
  const promptTokens = usage?.promptTokens || 0;
  const cachedTokens = usage?.promptCachedTokens || 0;
  const writeCachedTokens = usage?.promptWriteCachedTokens || 0;
  const reasoningTokens = usage?.completionReasoningTokens || 0;
  const cacheHitRate = promptTokens > 0 && cachedTokens > 0 ? ((cachedTokens / promptTokens) * 100).toFixed(1) : '0.0';
  const writeCacheRate = promptTokens > 0 && writeCachedTokens > 0 ? ((writeCachedTokens / promptTokens) * 100).toFixed(1) : '0.0';
  const formatCurrency = (value: number) =>
    t('currencies.format', {
      val: value,
      currency: settings?.currencyCode,
      locale: i18n.language === 'zh' ? 'zh-CN' : 'en-US',
      minimumFractionDigits: 6,
    });

  return (
    <div className='space-y-6'>
      <Card className='border-0 shadow-sm'>
        <CardHeader className='pb-2'>
          <CardTitle className='flex items-center justify-between'>
            <span className='flex items-center gap-2'><DashboardIcon className='text-primary h-4 w-4' />{t('requests.detail.overview')}</span>
            <Badge className={getStatusColor(request.status)} variant='secondary'>{t(`requests.status.${request.status}`)}</Badge>
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div className='grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-3'>
            <div className='bg-muted/30 flex items-center justify-between rounded-lg border p-3'><span className='flex items-center gap-2 text-sm'><Database className='text-primary h-4 w-4' />{t('requests.columns.channel')}</span><span className='font-mono text-sm'>{request.channel?.name || t('requests.columns.unknown')}</span></div>
            <div className='bg-muted/30 flex items-center justify-between rounded-lg border p-3'><span className='flex items-center gap-2 text-sm'><Database className='text-primary h-4 w-4' />{t('requests.columns.modelId')}</span><span className='font-mono text-sm'>{request.modelID || t('requests.columns.unknown')}</span></div>
            <div className='bg-muted/30 flex items-center justify-between rounded-lg border p-3'><span className='flex items-center gap-2 text-sm'><Key className='text-primary h-4 w-4' />{t('requests.dialogs.requestDetail.fields.apiKeyName')}</span><span className='font-mono text-sm'>{request.apiKey?.name || t('requests.columns.unknown')}</span></div>
          </div>
        </CardContent>
      </Card>

      {usage && (
        <Card className='border-0 shadow-sm'>
          <CardHeader className='pb-2'><CardTitle className='text-base'>{t('requests.detail.tabs.usage')}</CardTitle></CardHeader>
          <CardContent>
            <div className='grid grid-cols-2 gap-3 sm:grid-cols-4'>
              <div className='bg-muted/30 rounded-lg border p-3'>
                <p className='text-muted-foreground text-xs'>{t('usageLogs.columns.inputLabel')}</p>
                <p className='font-semibold'>{usage.promptTokens.toLocaleString()}</p>
              </div>
              <div className='bg-muted/30 rounded-lg border p-3'>
                <p className='text-muted-foreground text-xs'>{t('usageLogs.columns.outputLabel')}</p>
                <p className='font-semibold'>{usage.completionTokens.toLocaleString()}</p>
                {reasoningTokens > 0 && <p className='text-muted-foreground text-xs'>{t('requests.columns.reasoning')}: {reasoningTokens.toLocaleString()}</p>}
              </div>
              <div className='bg-muted/30 rounded-lg border p-3'>
                <p className='text-muted-foreground text-xs'>{t('usageLogs.columns.promptCachedTokens')}</p>
                <p className='font-semibold'>{cachedTokens.toLocaleString()}</p>
                {cachedTokens > 0 && <p className='text-muted-foreground text-xs'>{cacheHitRate}%</p>}
                {writeCachedTokens > 0 && <p className='text-muted-foreground text-xs'>{t('requests.columns.writeCache')}: {writeCachedTokens.toLocaleString()} ({writeCacheRate}%)</p>}
              </div>
              <div className='bg-muted/30 rounded-lg border p-3'>
                <p className='text-muted-foreground text-xs'>{t('requests.columns.cost')}</p>
                <p className='font-semibold'>{formatCurrency(usage.totalCost ?? 0)}</p>
              </div>
            </div>
          </CardContent>
        </Card>
      )}
    </div>
  );
}

function ExecutionSummariesPanel({ requestId, projectId, includeAdminFields }: { requestId: string; projectId?: string | null; includeAdminFields?: boolean }) {
  const { t, i18n } = useTranslation();
  const locale = i18n.language === 'zh' ? zhCN : enUS;
  const [expandedExecutionId, setExpandedExecutionId] = useState<string | null>(null);
  const { data, isLoading, isError, refetch } = useRequestExecutions(
    requestId,
    { first: 10, orderBy: { field: 'CREATED_AT', direction: 'DESC' } },
    { projectId, enabled: true }
  );

  if (isLoading) return <ContentLoading />;
  if (isError) return <ContentError retry={() => void refetch()} />;
  if (!data?.edges.length) return <div className='text-muted-foreground py-16 text-center'>{t('requests.dialogs.requestDetail.noExecutions')}</div>;

  return (
    <div className='space-y-4'>
      {data.edges.map(({ node: execution }, index) => (
        <Card key={execution.id} className='bg-muted/20 border-0 shadow-sm'>
          <CardHeader className='pb-3'>
            <CardTitle className='flex items-center justify-between text-base'>
              <span>{t('requests.dialogs.requestDetail.execution', { index: index + 1 })}</span>
              <Badge className={getStatusColor(execution.status)} variant='secondary'>{t(`requests.status.${execution.status}`)}</Badge>
            </CardTitle>
          </CardHeader>
          <CardContent className='space-y-3'>
            <div className='grid grid-cols-1 gap-3 sm:grid-cols-3'>
              <div className='bg-background rounded-lg border p-3'><p className='text-muted-foreground text-xs'>{t('requests.columns.channel')}</p><p className='font-mono text-sm'>{execution.channel?.name || t('requests.columns.unknown')}</p></div>
              <div className='bg-background rounded-lg border p-3'><p className='text-muted-foreground text-xs'>{t('requests.dialogs.requestDetail.fields.startTime')}</p><p className='font-mono text-sm'>{format(new Date(execution.createdAt), 'yyyy-MM-dd HH:mm:ss', { locale })}</p></div>
              <div className='bg-background rounded-lg border p-3'><p className='text-muted-foreground text-xs'>{t('requests.columns.firstTokenLatency')}</p><p className='font-mono text-sm'>{execution.metricsFirstTokenLatencyMs == null ? '-' : `${execution.metricsFirstTokenLatencyMs}ms`}</p></div>
            </div>
            {execution.errorMessage && <p className='text-destructive bg-destructive/10 rounded border p-3 text-sm'>{execution.errorMessage}</p>}
            <Button
              variant='outline'
              size='sm'
              onClick={() => setExpandedExecutionId((current) => nextExpandedExecution(current, execution.id))}
            >
              {expandedExecutionId === execution.id
                ? <ChevronDown className='mr-2 h-4 w-4' />
                : <ChevronRight className='mr-2 h-4 w-4' />}
              {expandedExecutionId === execution.id
                ? t('requests.detail.execution.hideContent')
                : t('requests.detail.execution.showContent')}
            </Button>
            {expandedExecutionId === execution.id && (
              <RequestExecutionContentPanel
                requestId={requestId}
                executionId={execution.id}
                projectId={projectId}
                includeAdminFields={includeAdminFields}
              />
            )}
          </CardContent>
        </Card>
      ))}
    </div>
  );
}

export function RequestDetailContent({
  request,
  requestId,
  projectId,
  includeAdminFields,
  activeTab,
  onActiveTabChange,
  previewRequest,
  previewChunks,
  previewVersion = 0,
  isPreviewStreaming = false,
}: RequestDetailContentProps) {
  const { t } = useTranslation();

  if (!request) return <ContentLoading />;

  return (
    <Card className='border-0 shadow-sm'>
      <CardContent className='p-0'>
        <Tabs value={activeTab} onValueChange={(value) => onActiveTabChange(value as RequestDetailTab)}>
          <div className='bg-muted/20 border-b px-6 pt-6'>
            <TabsList className='bg-background grid w-full grid-cols-4'>
              <TabsTrigger value='overview'>{t('requests.detail.tabs.overview')}</TabsTrigger>
              <TabsTrigger value='request'>{t('requests.detail.tabs.request')}</TabsTrigger>
              <TabsTrigger value='response'>{t('requests.detail.tabs.response')}</TabsTrigger>
              <TabsTrigger value='executions'>{t('requests.detail.tabs.executions')}</TabsTrigger>
            </TabsList>
          </div>

          <TabsContent value='overview' className='p-6'>
            {activeTab === 'overview' && <OverviewPanel request={request} />}
          </TabsContent>
          <TabsContent value='request' className='p-6'>
            {activeTab === 'request' && <RequestContentPanel request={request} requestId={requestId} projectId={projectId} includeAdminFields={includeAdminFields} />}
          </TabsContent>
          <TabsContent value='response' className='p-6'>
            {activeTab === 'response' && (
              <ResponseContentPanel
                request={request}
                requestId={requestId}
                projectId={projectId}
                includeAdminFields={includeAdminFields}
                previewRequest={previewRequest}
                previewChunks={previewChunks}
                previewVersion={previewVersion}
                isPreviewStreaming={isPreviewStreaming}
              />
            )}
          </TabsContent>
          <TabsContent value='executions' className='p-6'>
            {activeTab === 'executions' && <ExecutionSummariesPanel requestId={requestId} projectId={projectId} includeAdminFields={includeAdminFields} />}
          </TabsContent>
        </Tabs>
      </CardContent>
    </Card>
  );
}
