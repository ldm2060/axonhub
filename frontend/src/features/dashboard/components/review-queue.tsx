import { useState } from 'react';
import { ClipboardCheck, CheckCircle2, XCircle, Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { usePermissions } from '@/hooks/usePermissions';
import { formatUserName } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent } from '@/components/ui/card';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Skeleton } from '@/components/ui/skeleton';
import { Textarea } from '@/components/ui/textarea';
import { usePendingPublishRequests, useReviewPublishRequest } from '../data/publish-requests';
import { CollapsibleSection } from './collapsible-section';

function ReviewDialog({
  open,
  onOpenChange,
  action,
  requestId,
  resourceName,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  action: 'approve' | 'reject';
  requestId: string;
  resourceName: string;
}) {
  const { t } = useTranslation();
  const [comment, setComment] = useState('');
  const reviewMutation = useReviewPublishRequest();

  const handleSubmit = () => {
    reviewMutation.mutate(
      { id: requestId, action, comment: comment.trim() || undefined },
      {
        onSuccess: () => {
          onOpenChange(false);
          setComment('');
        },
      }
    );
  };

  const isApprove = action === 'approve';

  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>
            {isApprove ? t('publish.approve') : t('publish.reject')} - {resourceName}
          </DialogTitle>
          <DialogDescription>{t('publish.reviewComment')}</DialogDescription>
        </DialogHeader>
        <Textarea
          value={comment}
          onChange={(e) => setComment(e.target.value)}
          placeholder={t('publish.reviewCommentPlaceholder')}
          rows={3}
        />
        <DialogFooter>
          <Button variant='outline' onClick={() => onOpenChange(false)}>
            {t('common.buttons.cancel')}
          </Button>
          <Button variant={isApprove ? 'default' : 'destructive'} onClick={handleSubmit} disabled={reviewMutation.isPending}>
            {reviewMutation.isPending && <Loader2 className='mr-2 h-4 w-4 animate-spin' />}
            {isApprove ? t('publish.approve') : t('publish.reject')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

export function ReviewQueue() {
  const { t } = useTranslation();
  const { hasScope } = usePermissions();
  const canReview = hasScope('review_publish_requests');

  const { data: requests, isLoading } = usePendingPublishRequests({
    enabled: canReview,
  });

  const [dialogState, setDialogState] = useState<{
    open: boolean;
    action: 'approve' | 'reject';
    requestId: string;
    resourceName: string;
  }>({
    open: false,
    action: 'approve',
    requestId: '',
    resourceName: '',
  });

  if (!canReview) {
    return null;
  }

  const getResourceName = (req: { resourceType: string; resourceID: number }) => {
    const type = req.resourceType === 'channel' ? t('dashboard.stats.channel') : t('dashboard.stats.model');
    return `${type} #${req.resourceID}`;
  };

  const formatDate = (dateStr: string) => {
    try {
      return new Date(dateStr).toLocaleDateString(undefined, {
        year: 'numeric',
        month: 'short',
        day: 'numeric',
        hour: '2-digit',
        minute: '2-digit',
      });
    } catch {
      return dateStr;
    }
  };

  return (
    <>
      <CollapsibleSection
        title={t('dashboard.reviewQueue')}
        icon={<ClipboardCheck className='text-primary h-4 w-4' />}
        storageKey='reviewQueue'
      >
        {isLoading ? (
          <Card>
            <CardContent className='p-4'>
              <div className='space-y-3'>
                <Skeleton className='h-12 w-full' />
                <Skeleton className='h-12 w-full' />
              </div>
            </CardContent>
          </Card>
        ) : !requests || requests.length === 0 ? (
          <Card>
            <CardContent className='flex flex-col items-center justify-center py-8'>
              <ClipboardCheck className='text-muted-foreground mb-2 h-8 w-8' />
              <p className='text-muted-foreground text-sm'>{t('dashboard.noPendingRequests')}</p>
            </CardContent>
          </Card>
        ) : (
          <Card>
            <CardContent className='p-0'>
              <div className='divide-y'>
                {requests.map((req) => (
                  <div key={req.id} className='flex items-center justify-between gap-4 px-4 py-3'>
                    <div className='min-w-0 flex-1'>
                      <div className='flex items-center gap-2'>
                        <Badge variant='outline' className='text-xs'>
                          {req.resourceType === 'channel' ? t('dashboard.stats.channel') : t('dashboard.stats.model')}
                        </Badge>
                        <span className='text-sm font-medium'>#{req.resourceID}</span>
                      </div>
                      <div className='text-muted-foreground mt-1 flex items-center gap-2 text-xs'>
                        <span>
                          {formatUserName(req.requester.firstName, req.requester.lastName)}
                        </span>
                        <span>-</span>
                        <span>{formatDate(req.createdAt)}</span>
                        {req.requestComment && (
                          <>
                            <span>-</span>
                            <span className='truncate' title={req.requestComment}>
                              {req.requestComment}
                            </span>
                          </>
                        )}
                      </div>
                    </div>
                    <div className='flex shrink-0 items-center gap-2'>
                      <Button
                        size='sm'
                        variant='outline'
                        className='h-8 text-green-600 hover:bg-green-50 hover:text-green-700 dark:text-green-400 dark:hover:bg-green-950'
                        onClick={() =>
                          setDialogState({
                            open: true,
                            action: 'approve',
                            requestId: req.id,
                            resourceName: getResourceName(req),
                          })
                        }
                      >
                        <CheckCircle2 className='mr-1 h-3.5 w-3.5' />
                        {t('publish.approve')}
                      </Button>
                      <Button
                        size='sm'
                        variant='outline'
                        className='h-8 text-red-600 hover:bg-red-50 hover:text-red-700 dark:text-red-400 dark:hover:bg-red-950'
                        onClick={() =>
                          setDialogState({
                            open: true,
                            action: 'reject',
                            requestId: req.id,
                            resourceName: getResourceName(req),
                          })
                        }
                      >
                        <XCircle className='mr-1 h-3.5 w-3.5' />
                        {t('publish.reject')}
                      </Button>
                    </div>
                  </div>
                ))}
              </div>
            </CardContent>
          </Card>
        )}
      </CollapsibleSection>

      <ReviewDialog
        open={dialogState.open}
        onOpenChange={(open) => setDialogState((prev) => ({ ...prev, open }))}
        action={dialogState.action}
        requestId={dialogState.requestId}
        resourceName={dialogState.resourceName}
      />
    </>
  );
}
