import { format } from 'date-fns';
import { useParams, useNavigate, useRouter } from '@tanstack/react-router';
import { ArrowLeft, Copy, FileText } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { copyTextToClipboard } from '@/lib/clipboard';
import { extractNumberID } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Separator } from '@/components/ui/separator';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { useRequest } from '../data';
import { RequestDetailContent } from './request-detail-content';

interface RequestDetailGlobalPageProps {
  /** Explicit back target for scopes with their own list page (e.g. /admin/requests). */
  backTo?: '/admin/channels' | '/admin/requests';
}

export default function RequestDetailGlobalPage({ backTo }: RequestDetailGlobalPageProps) {
  const { t } = useTranslation();
  // Rendered from both /requests/$requestId and /admin/requests/$requestId,
  // so the route cannot be pinned via `from`.
  const { requestId } = useParams({ strict: false }) as { requestId: string };
  const router = useRouter();
  const navigate = useNavigate();
  const { data: request } = useRequest(requestId, { projectId: null });

  const handleBack = () => {
    if (backTo) {
      void navigate({ to: backTo });
      return;
    }
    router.history.back();
  };

  const copyRequestID = async () => {
    try {
      await copyTextToClipboard(request?.id ?? requestId);
      toast.success(t('requests.actions.copied'));
    } catch {
      toast.error(t('common.errors.copyFailed'));
    }
  };

  return (
    <div className='flex h-full flex-col'>
      <Header className='bg-background/95 supports-[backdrop-filter]:bg-background/60 border-b backdrop-blur'>
        <div className='flex items-center space-x-4'>
          <Button variant='ghost' size='sm' onClick={handleBack} className='hover:bg-accent'>
            <ArrowLeft className='mr-2 h-4 w-4' />
            {t('common.back')}
          </Button>
          <Separator orientation='vertical' className='h-6' />
          <div className='flex items-center space-x-3'>
            <div className='bg-primary/10 flex h-8 w-8 items-center justify-center rounded-lg'>
              <FileText className='text-primary h-4 w-4' />
            </div>
            <div>
              <div className='flex items-center gap-1'>
                <h1 className='text-lg leading-none font-semibold'>
                  {t('requests.detail.title')} #{request ? extractNumberID(request.id) || request.id : extractNumberID(requestId) || requestId}
                </h1>
                <Tooltip>
                  <TooltipTrigger asChild>
                    <Button variant='ghost' size='icon-sm' className='h-7 w-7' onClick={() => void copyRequestID()} aria-label={t('requests.actions.copyRequestId')}>
                      <Copy className='h-3.5 w-3.5' />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent>{t('requests.actions.copyRequestId')}</TooltipContent>
                </Tooltip>
              </div>
              {request && (
                <div className='mt-1 flex items-center gap-2'>
                  <p className='text-muted-foreground text-sm'>{request.modelID || t('requests.columns.unknown')}</p>
                  <span className='text-muted-foreground text-xs'>•</span>
                  <p className='text-muted-foreground text-xs'>{format(new Date(request.createdAt), 'yyyy-MM-dd HH:mm:ss')}</p>
                </div>
              )}
            </div>
          </div>
        </div>
      </Header>

      <Main className='flex-1 overflow-auto'>
        <div className='container mx-auto max-w-7xl p-6'>
          <RequestDetailContent requestId={requestId} projectId={null} />
        </div>
      </Main>
    </div>
  );
}
