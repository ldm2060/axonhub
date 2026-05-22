'use client';

import { useState, useCallback } from 'react';
import { IconShare, IconX, IconLoader2, IconUserPlus } from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { ScrollArea } from '@/components/ui/scroll-area';
import { AutoCompleteSelect } from '@/components/auto-complete-select';
import { useShareChannel, useUnshareChannel, useRequestPublish } from '@/gql/sharing';
import { useUsers } from '@/features/users/data/users';
import { useAuthStore } from '@/stores/authStore';
import { Channel } from '../data/schema';

interface Props {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  channel: Channel;
}

export function ChannelsShareDialog({ open, onOpenChange, channel }: Props) {
  const { t } = useTranslation();
  const { user: authUser } = useAuthStore((state) => state.auth);
  const shareChannel = useShareChannel();
  const unshareChannel = useUnshareChannel();
  const requestPublish = useRequestPublish();

  const [selectedUserId, setSelectedUserId] = useState<string>('');
  const [publishComment, setPublishComment] = useState('');
  const [showPublishDialog, setShowPublishDialog] = useState(false);

  // Fetch all users for the sharing dropdown
  const { data: usersData } = useUsers(
    { first: 100, where: { statusIn: ['activated'] } },
    { disableAutoFetch: false }
  );

  const isOwner = !channel.ownerID || channel.ownerID === String(authUser?.id) || authUser?.isOwner;
  const sharedWith = channel.sharedWith || [];
  const visibility = channel.visibility || 'private';

  // Build user options, excluding already shared users and the owner
  const userOptions = (usersData?.edges || [])
    .map((edge) => ({
      value: edge.node.id,
      label: `${edge.node.firstName} ${edge.node.lastName} (${edge.node.email})`.trim(),
    }))
    .filter((opt) => !sharedWith.includes(Number(opt.value)) && opt.value !== String(authUser?.id));

  // Build shared users list
  const sharedUsers = sharedWith
    .map((userId) => {
      const user = usersData?.edges?.find((edge) => edge.node.id === String(userId));
      return user
        ? { id: String(userId), name: `${user.node.firstName} ${user.node.lastName}`.trim(), email: user.node.email }
        : { id: String(userId), name: String(userId), email: '' };
    });

  const handleShare = useCallback(async () => {
    if (!selectedUserId) return;
    try {
      await shareChannel.mutateAsync({ id: channel.id, userIDs: [selectedUserId] });
      setSelectedUserId('');
    } catch {
      // Error handled by mutation
    }
  }, [selectedUserId, channel.id, shareChannel]);

  const handleUnshare = useCallback(
    async (userId: string) => {
      try {
        await unshareChannel.mutateAsync({ id: channel.id, userIDs: [userId] });
      } catch {
        // Error handled by mutation
      }
    },
    [channel.id, unshareChannel]
  );

  const handleRequestPublish = useCallback(async () => {
    try {
      await requestPublish.mutateAsync({
        resourceType: 'channel',
        resourceID: channel.id,
        comment: publishComment || undefined,
      });
      setShowPublishDialog(false);
      setPublishComment('');
    } catch {
      // Error handled by mutation
    }
  }, [channel.id, publishComment, requestPublish]);

  const getVisibilityBadge = () => {
    const variant =
      visibility === 'published'
        ? 'default'
        : visibility === 'shared'
          ? 'secondary'
          : 'outline';
    return (
      <Badge variant={variant} className='text-xs'>
        {t(`share.visibility.${visibility}`)}
      </Badge>
    );
  };

  return (
    <>
      <Dialog open={open && !showPublishDialog} onOpenChange={onOpenChange}>
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle className='flex items-center gap-2'>
              <IconShare className='h-5 w-5' />
              {t('share.dialog.title', { name: channel.name })}
            </DialogTitle>
            <DialogDescription>{t('share.dialog.description.channel')}</DialogDescription>
          </DialogHeader>

          <div className='space-y-4'>
            {/* Visibility Status */}
            <div className='flex items-center gap-2'>
              <span className='text-sm font-medium'>{t('share.dialog.visibility')}:</span>
              {getVisibilityBadge()}
            </div>

            {isOwner && (
              <>
                {/* Add User Section */}
                <div className='space-y-2'>
                  <label className='text-sm font-medium'>{t('share.dialog.addUser')}</label>
                  <div className='flex gap-2'>
                    <div className='flex-1'>
                      <AutoCompleteSelect
                        selectedValue={selectedUserId}
                        onSelectedValueChange={setSelectedUserId}
                        items={userOptions}
                        isLoading={false}
                        emptyMessage={t('share.dialog.noUsers')}
                        placeholder={t('share.dialog.searchUsers')}
                      />
                    </div>
                    <Button
                      onClick={handleShare}
                      disabled={!selectedUserId || shareChannel.isPending}
                      size='sm'
                    >
                      {shareChannel.isPending ? (
                        <IconLoader2 className='h-4 w-4 animate-spin' />
                      ) : (
                        <IconUserPlus className='h-4 w-4' />
                      )}
                    </Button>
                  </div>
                </div>

                {/* Shared Users List */}
                {sharedUsers.length > 0 && (
                  <div className='space-y-2'>
                    <label className='text-sm font-medium'>{t('share.dialog.sharedWith')}</label>
                    <ScrollArea className='max-h-[200px]'>
                      <div className='space-y-1'>
                        {sharedUsers.map((user) => (
                          <div
                            key={user.id}
                            className='flex items-center justify-between rounded-md border px-3 py-2'
                          >
                            <div className='min-w-0 flex-1'>
                              <p className='text-sm font-medium truncate'>{user.name}</p>
                              {user.email && (
                                <p className='text-xs text-muted-foreground truncate'>{user.email}</p>
                              )}
                            </div>
                            <Button
                              variant='ghost'
                              size='sm'
                              className='ml-2 h-7 w-7 p-0 text-muted-foreground hover:text-destructive'
                              onClick={() => handleUnshare(user.id)}
                              disabled={unshareChannel.isPending}
                            >
                              <IconX className='h-4 w-4' />
                            </Button>
                          </div>
                        ))}
                      </div>
                    </ScrollArea>
                  </div>
                )}

                {/* Request Publish Button */}
                {visibility !== 'published' && (
                  <div className='pt-2'>
                    <Button
                      variant='outline'
                      className='w-full'
                      onClick={() => setShowPublishDialog(true)}
                    >
                      <IconShare className='mr-2 h-4 w-4' />
                      {t('share.dialog.requestPublish')}
                    </Button>
                  </div>
                )}
              </>
            )}

            {!isOwner && (
              <p className='text-sm text-muted-foreground'>{t('share.dialog.notOwner')}</p>
            )}
          </div>

          <DialogFooter>
            <Button variant='outline' onClick={() => onOpenChange(false)}>
              {t('common.buttons.close')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      {/* Publish Request Sub-dialog */}
      <Dialog open={showPublishDialog} onOpenChange={setShowPublishDialog}>
        <DialogContent className='sm:max-w-md'>
          <DialogHeader>
            <DialogTitle>{t('share.dialog.requestPublishTitle')}</DialogTitle>
            <DialogDescription>{t('share.dialog.requestPublishDescription.channel', { name: channel.name })}</DialogDescription>
          </DialogHeader>
          <div className='space-y-4'>
            <div className='space-y-2'>
              <label className='text-sm font-medium'>{t('share.dialog.publishComment')}</label>
              <textarea
                className='flex min-h-[80px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 disabled:cursor-not-allowed disabled:opacity-50'
                placeholder={t('share.dialog.publishCommentPlaceholder')}
                value={publishComment}
                onChange={(e) => setPublishComment(e.target.value)}
              />
            </div>
          </div>
          <DialogFooter>
            <Button variant='outline' onClick={() => setShowPublishDialog(false)}>
              {t('common.buttons.cancel')}
            </Button>
            <Button onClick={handleRequestPublish} disabled={requestPublish.isPending}>
              {requestPublish.isPending && <IconLoader2 className='mr-2 h-4 w-4 animate-spin' />}
              {t('share.dialog.submitPublishRequest')}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </>
  );
}
