'use client';

import { useState, useCallback } from 'react';
import { format } from 'date-fns';
import { IconCheck, IconX, IconLoader2 } from '@tabler/icons-react';
import { usePublishRequests, useCancelPublishRequest, useReviewPublishRequest } from '@/gql/sharing';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '@/stores/authStore';
import { usePermissions } from '@/hooks/usePermissions';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '@/components/ui/table';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';

function getStatusBadge(status: string, t: (key: string) => string) {
  const variants: Record<string, 'default' | 'secondary' | 'outline' | 'destructive'> = {
    pending: 'outline',
    approved: 'default',
    rejected: 'destructive',
  };
  return <Badge variant={variants[status] || 'outline'}>{t(`publishRequests.status.${status}`)}</Badge>;
}

export default function PublishRequestsPage() {
  const { t } = useTranslation();
  const { publishRequestPermissions } = usePermissions();
  const { user: authUser } = useAuthStore((state) => state.auth);
  const canReview = publishRequestPermissions.canReview;
  const cancelRequest = useCancelPublishRequest();
  const reviewRequest = useReviewPublishRequest();

  const [reviewDialog, setReviewDialog] = useState<{
    id: string;
    action: 'approve' | 'reject';
    resourceName: string;
  } | null>(null);
  const [reviewComment, setReviewComment] = useState('');

  const { data, isLoading } = usePublishRequests({
    first: 50,
    orderBy: { field: 'CREATED_AT', direction: 'DESC' },
  });

  const requests = data?.edges?.map((edge) => edge.node) || [];

  const handleCancel = useCallback(
    async (id: string) => {
      try {
        await cancelRequest.mutateAsync(id);
      } catch {
        // Error handled by mutation
      }
    },
    [cancelRequest]
  );

  const handleReview = useCallback(async () => {
    if (!reviewDialog) return;
    try {
      await reviewRequest.mutateAsync({
        id: reviewDialog.id,
        action: reviewDialog.action,
        comment: reviewComment || undefined,
      });
      setReviewDialog(null);
      setReviewComment('');
    } catch {
      // Error handled by mutation
    }
  }, [reviewDialog, reviewComment, reviewRequest]);

  const getResourceName = useCallback(
    (request: (typeof requests)[0]) => {
      return `${t(`publishRequests.resourceType.${request.resourceType}`)} #${request.resourceID}`;
    },
    [t]
  );

  return (
    <>
      <Header fixed>
        <div className='flex flex-1 items-center justify-between'>
          <div>
            <h2 className='text-xl font-bold tracking-tight'>{t('publishRequests.title')}</h2>
            <p className='text-muted-foreground text-sm'>{t('publishRequests.description')}</p>
          </div>
        </div>
      </Header>

      <Main fixed>
        <div className='flex flex-1 flex-col overflow-hidden'>
          <ScrollArea className='flex-1'>
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>{t('publishRequests.columns.resourceType')}</TableHead>
                  <TableHead>{t('publishRequests.columns.resourceID')}</TableHead>
                  <TableHead>{t('publishRequests.columns.requester')}</TableHead>
                  <TableHead>{t('publishRequests.columns.status.label')}</TableHead>
                  <TableHead>{t('publishRequests.columns.comment')}</TableHead>
                  <TableHead>{t('publishRequests.columns.createdAt')}</TableHead>
                  <TableHead>{t('publishRequests.columns.actions')}</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {isLoading && (
                  <TableRow>
                    <TableCell colSpan={7} className='h-24 text-center'>
                      <IconLoader2 className='text-muted-foreground mx-auto h-6 w-6 animate-spin' />
                    </TableCell>
                  </TableRow>
                )}
                {!isLoading && requests.length === 0 && (
                  <TableRow>
                    <TableCell colSpan={7} className='text-muted-foreground h-24 text-center'>
                      {t('publishRequests.empty')}
                    </TableCell>
                  </TableRow>
                )}
                {requests.map((request) => {
                  const isRequester = request.requesterID === String(authUser?.id);
                  const canCancel = isRequester && request.status === 'pending';
                  const canApproveReject = canReview && request.status === 'pending';

                  return (
                    <TableRow key={request.id}>
                      <TableCell>
                        <Badge variant='secondary' className='text-xs'>
                          {t(`publishRequests.resourceType.${request.resourceType}`)}
                        </Badge>
                      </TableCell>
                      <TableCell className='font-mono text-sm'>{request.resourceID}</TableCell>
                      <TableCell>
                        <div>
                          <p className='text-sm font-medium'>
                            {request.requester.firstName} {request.requester.lastName}
                          </p>
                          <p className='text-muted-foreground text-xs'>{request.requester.email}</p>
                        </div>
                      </TableCell>
                      <TableCell>{getStatusBadge(request.status, t)}</TableCell>
                      <TableCell className='max-w-[200px] truncate text-sm'>{request.requestComment || '-'}</TableCell>
                      <TableCell className='text-sm'>{format(new Date(request.createdAt), 'yyyy-MM-dd HH:mm')}</TableCell>
                      <TableCell>
                        <div className='flex items-center gap-1'>
                          {canCancel && (
                            <Button
                              variant='outline'
                              size='sm'
                              className='h-7 text-xs'
                              onClick={() => handleCancel(request.id)}
                              disabled={cancelRequest.isPending}
                            >
                              {t('publishRequests.actions.cancel')}
                            </Button>
                          )}
                          {canApproveReject && (
                            <>
                              <Button
                                variant='outline'
                                size='sm'
                                className='h-7 text-xs text-green-600 hover:text-green-700'
                                onClick={() =>
                                  setReviewDialog({
                                    id: request.id,
                                    action: 'approve',
                                    resourceName: getResourceName(request),
                                  })
                                }
                                disabled={reviewRequest.isPending}
                              >
                                <IconCheck className='mr-1 h-3 w-3' />
                                {t('publishRequests.actions.approve')}
                              </Button>
                              <Button
                                variant='outline'
                                size='sm'
                                className='h-7 text-xs text-red-600 hover:text-red-700'
                                onClick={() =>
                                  setReviewDialog({
                                    id: request.id,
                                    action: 'reject',
                                    resourceName: getResourceName(request),
                                  })
                                }
                                disabled={reviewRequest.isPending}
                              >
                                <IconX className='mr-1 h-3 w-3' />
                                {t('publishRequests.actions.reject')}
                              </Button>
                            </>
                          )}
                          {request.status !== 'pending' && request.reviewer && (
                            <span className='text-muted-foreground text-xs'>
                              {t('publishRequests.columns.reviewedBy', {
                                name: `${request.reviewer.firstName} ${request.reviewer.lastName}`,
                              })}
                            </span>
                          )}
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </ScrollArea>
        </div>
      </Main>

      {/* Review Dialog */}
      <Dialog
        open={!!reviewDialog}
        onOpenChange={(isOpen) => {
          if (!isOpen) {
            setReviewDialog(null);
            setReviewComment('');
          }
        }}
      >
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>
              {reviewDialog?.action === 'approve' ? t('publishRequests.dialog.approveTitle') : t('publishRequests.dialog.rejectTitle')}
            </DialogTitle>
            <DialogDescription>
              {reviewDialog?.action === 'approve'
                ? t('publishRequests.dialog.approveDescription', { resource: reviewDialog?.resourceName })
                : t('publishRequests.dialog.rejectDescription', { resource: reviewDialog?.resourceName })}
            </DialogDescription>
          </DialogHeader>
          <div className='space-y-2'>
            <label className='text-sm font-medium'>{t('publishRequests.dialog.reviewComment')}</label>
            <textarea
              className='border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring flex min-h-[80px] w-full rounded-md border px-3 py-2 text-sm focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50'
              placeholder={t('publishRequests.dialog.reviewCommentPlaceholder')}
              value={reviewComment}
              onChange={(e) => setReviewComment(e.target.value)}
            />
          </div>
          <DialogFooter>
            <Button
              variant='outline'
              onClick={() => {
                setReviewDialog(null);
                setReviewComment('');
              }}
            >
              {t('common.buttons.cancel')}
            </Button>
            <Button
              onClick={handleReview}
              disabled={reviewRequest.isPending}
              variant={reviewDialog?.action === 'reject' ? 'destructive' : 'default'}
            >
              {reviewRequest.isPending && <IconLoader2 className='mr-2 h-4 w-4 animate-spin' />}
              {reviewDialog?.action === 'approve' ? t('publishRequests.actions.approve') : t('publishRequests.actions.reject')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
