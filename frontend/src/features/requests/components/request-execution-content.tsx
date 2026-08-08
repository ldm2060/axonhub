import { useState } from 'react';
import { Copy, Download, FileText, Layers, Terminal } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { JsonViewer } from '@/components/json-tree-view';
import { useRequestExecutionContent } from '../data';
import { generateExecutionCurl } from '../utils/curl-generator';
import { ChunksDialog } from './chunks-dialog';
import { CurlPreviewDialog } from './curl-preview-dialog';

interface RequestExecutionContentPanelProps {
  requestId: string;
  executionId: string;
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

function downloadFile(content: string, filename: string) {
  const blob = new Blob([content], { type: 'application/json' });
  const url = URL.createObjectURL(blob);
  const anchor = document.createElement('a');
  anchor.href = url;
  anchor.download = filename;
  anchor.click();
  URL.revokeObjectURL(url);
}

export function RequestExecutionContentPanel({ requestId, executionId, projectId, includeAdminFields }: RequestExecutionContentPanelProps) {
  const { t } = useTranslation();
  const { data, isLoading, isError, refetch } = useRequestExecutionContent(requestId, executionId, {
    projectId,
    includeAdminFields,
    enabled: true,
  });
  const [showChunks, setShowChunks] = useState(false);
  const [showCurl, setShowCurl] = useState(false);
  const [curlCommand, setCurlCommand] = useState('');

  if (isLoading) {
    return (
      <div className='flex min-h-64 items-center justify-center'>
        <div className='border-primary h-8 w-8 animate-spin rounded-full border-b-2' />
      </div>
    );
  }

  if (isError) {
    return (
      <div className='space-y-4 py-10 text-center'>
        <p className='text-muted-foreground'>{t('requests.detail.loadFailed')}</p>
        <Button variant='outline' onClick={() => void refetch()}>
          {t('requests.detail.retry')}
        </Button>
      </div>
    );
  }

  if (!data) return null;

  const copy = (value: any) => {
    void navigator.clipboard.writeText(formatJson(value));
    toast.success(t('requests.actions.copy'));
  };

  return (
    <div className='mt-4 space-y-6 border-t pt-6'>
      {(data.requestHeaders || data.requestBody) && (
        <div className='flex justify-end'>
          <Button
            variant='outline'
            size='sm'
            onClick={() => {
              setCurlCommand(
                generateExecutionCurl(
                  data.requestHeaders,
                  data.requestBody,
                  data.channel ?? undefined,
                  data.format,
                  data.requestURL ?? undefined
                )
              );
              setShowCurl(true);
            }}
          >
            <Terminal className='mr-2 h-4 w-4' />
            {t('requests.actions.copyCurl')}
          </Button>
        </div>
      )}

      {(
        [
          ['requestHeaders', t('requests.columns.requestHeaders'), data.requestHeaders, 'request-headers'],
          ['requestBody', t('requests.columns.requestBody'), data.requestBody, 'request-body'],
          ['responseBody', t('requests.columns.responseBody'), data.responseBody, 'response-body'],
        ] as const
      ).map(
        ([key, label, value, filename]) =>
          value && (
            <section key={key} className='space-y-3'>
              <div className='flex items-center justify-between gap-4'>
                <h6 className='flex items-center gap-2 font-semibold'>
                  <FileText className='text-primary h-4 w-4' />
                  {label}
                </h6>
                <div className='flex gap-2'>
                  {key === 'responseBody' && (
                    <Button variant='outline' size='sm' disabled={!data.responseChunks?.length} onClick={() => setShowChunks(true)}>
                      <Layers className='mr-2 h-4 w-4' />
                      {t('requests.columns.responseChunks')}
                    </Button>
                  )}
                  <Button variant='outline' size='sm' onClick={() => copy(value)}>
                    <Copy className='mr-2 h-4 w-4' />
                    {t('requests.dialogs.jsonViewer.copy')}
                  </Button>
                  <Button
                    variant='outline'
                    size='sm'
                    onClick={() => downloadFile(formatJson(value), `execution-${executionId}-${filename}.json`)}
                  >
                    <Download className='mr-2 h-4 w-4' />
                    {t('requests.dialogs.jsonViewer.download')}
                  </Button>
                </div>
              </div>
              <div className='bg-background h-72 overflow-auto rounded-lg border p-3'>
                <JsonViewer data={value} rootName='' defaultExpanded={false} hideArrayIndices className='text-xs' />
              </div>
            </section>
          )
      )}

      <ChunksDialog
        open={showChunks}
        onOpenChange={setShowChunks}
        chunks={data.responseChunks ?? []}
        title={t('requests.dialogs.jsonViewer.responseChunks')}
      />
      <CurlPreviewDialog
        open={showCurl}
        onOpenChange={(open) => {
          setShowCurl(open);
          if (!open) setCurlCommand('');
        }}
        curlCommand={curlCommand}
      />
    </div>
  );
}
