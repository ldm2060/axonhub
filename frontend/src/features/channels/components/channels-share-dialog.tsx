'use client';

import { useState, useCallback, useMemo, useEffect } from 'react';
import { IconShare, IconX, IconLoader2, IconUserPlus } from '@tabler/icons-react';
import { useShareChannel, useUnshareChannel, useRequestPublish, useShareableUsers, type SharedUser } from '@/gql/sharing';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '@/stores/authStore';
import { buildGUID, extractNumberID, formatUserName } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { ScrollArea } from '@/components/ui/scroll-area';
import { AutoCompleteSelect } from '@/components/auto-complete-select';
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
  // The dialog is handed the table row it was opened from, which goes stale as soon
  // as a share mutation lands. Keep the server's answer and prefer it over the row.
  const [sharedState, setSharedState] = useState<{
    sharedWith: number[];
    sharedUsers: SharedUser[];
    visibility: string;
  } | null>(null);

  const isOwner = !channel.ownerID || channel.ownerID === String(authUser?.id) || authUser?.isOwner;
  // shared_with stores bare user IDs while every user ID that reaches the UI is a
  // GUID, so compare on the numeric part of either shape. Getting this wrong is
  // what made sharing look broken (see .agent/rules/channel-access.md).
  const sharedWithIds = useMemo(() => sharedState?.sharedWith ?? channel.sharedWith ?? [], [sharedState, channel.sharedWith]);
  const sharedUserList = useMemo(() => sharedState?.sharedUsers ?? channel.sharedUsers ?? [], [sharedState, channel.sharedUsers]);
  const visibility = sharedState?.visibility ?? channel.visibility ?? 'private';

  useEffect(() => {
    setSharedState(null);
  }, [channel.id, open]);

  const { data: shareableUsers = [], isLoading: isLoadingShareableUsers } = useShareableUsers({
    disableAutoFetch: !open || !isOwner,
  });

  // Build user options, excluding already shared users and the current user
  const userOptions = useMemo(
    () =>
      shareableUsers
        .filter((user) => !sharedWithIds.includes(Number(extractNumberID(user.id))) && user.id !== authUser?.id)
        .map((user) => ({
          value: user.id,
          label: `${formatUserName(user.firstName, user.lastName) || user.email} (${user.email})`,
        })),
    [shareableUsers, sharedWithIds, authUser?.id]
  );

  // Resolved names come from the server so the list renders even when the caller
  // cannot list users. A user that no longer exists is still listed, by its ID, so
  // the owner can drop it from shared_with.
  const sharedUsers = useMemo(() => {
    const resolved = sharedUserList.map((user) => ({
      id: user.id,
      userId: String(extractNumberID(user.id)),
      name: formatUserName(user.firstName, user.lastName) || user.email,
      email: user.email,
    }));
    const resolvedIds = resolved.map((user) => user.userId);
    const unresolved = sharedWithIds
      .map(String)
      .filter((userId) => !resolvedIds.includes(userId))
      .map((userId) => ({
        id: buildGUID('User', userId),
        userId,
        name: t('share.dialog.unknownUser', { id: userId }),
        email: '',
      }));

    return [...resolved, ...unresolved];
  }, [sharedUserList, sharedWithIds, t]);

  const handleShare = useCallback(async () => {
    if (!selectedUserId) return;
    try {
      const updated = await shareChannel.mutateAsync({ id: channel.id, userIDs: [selectedUserId] });
      setSharedState({ sharedWith: updated.sharedWith, sharedUsers: updated.sharedUsers ?? [], visibility: updated.visibility });
      setSelectedUserId('');
    } catch {
      // Error handled by mutation
    }
  }, [selectedUserId, channel.id, shareChannel]);

  const handleUnshare = useCallback(
    async (userId: string) => {
      try {
        const updated = await unshareChannel.mutateAsync({ id: channel.id, userIDs: [userId] });
        setSharedState({ sharedWith: updated.sharedWith, sharedUsers: updated.sharedUsers ?? [], visibility: updated.visibility });
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
    const variant = visibility === 'published' ? 'default' : visibility === 'shared' ? 'secondary' : 'outline';
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
                        isLoading={isLoadingShareableUsers}
                        emptyMessage={t('share.dialog.noUsers')}
                        placeholder={t('share.dialog.searchUsers')}
                      />
                    </div>
                    <Button onClick={handleShare} disabled={!selectedUserId || shareChannel.isPending} size='sm'>
                      {shareChannel.isPending ? <IconLoader2 className='h-4 w-4 animate-spin' /> : <IconUserPlus className='h-4 w-4' />}
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
                          <div key={user.id} className='flex items-center justify-between rounded-md border px-3 py-2'>
                            <div className='min-w-0 flex-1'>
                              <p className='truncate text-sm font-medium'>{user.name}</p>
                              {user.email && <p className='text-muted-foreground truncate text-xs'>{user.email}</p>}
                            </div>
                            <Button
                              variant='ghost'
                              size='sm'
                              className='text-muted-foreground hover:text-destructive ml-2 h-7 w-7 p-0'
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
                    <Button variant='outline' className='w-full' onClick={() => setShowPublishDialog(true)}>
                      <IconShare className='mr-2 h-4 w-4' />
                      {t('share.dialog.requestPublish')}
                    </Button>
                  </div>
                )}
              </>
            )}

            {!isOwner && <p className='text-muted-foreground text-sm'>{t('share.dialog.notOwner')}</p>}
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
                className='border-input bg-background ring-offset-background placeholder:text-muted-foreground focus-visible:ring-ring flex min-h-[80px] w-full rounded-md border px-3 py-2 text-sm focus-visible:ring-2 focus-visible:ring-offset-2 focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-50'
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
