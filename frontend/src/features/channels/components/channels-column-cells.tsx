import { memo, useCallback, useEffect, useRef, useState } from 'react';
import { format } from 'date-fns';
import { DotsHorizontalIcon } from '@radix-ui/react-icons';
import type { Row } from '@tanstack/react-table';
import {
  IconPlayerPlay,
  IconChevronDown,
  IconChevronRight,
  IconAlertTriangle,
  IconEdit,
  IconArchive,
  IconTrash,
  IconCheck,
  IconTransform,
  IconNetwork,
  IconAdjustments,
  IconRoute,
  IconCopy,
  IconCoin,
  IconLoader2,
  IconKey,
  IconKeyOff,
  IconGauge,
  IconHistory,
  IconPlugConnected,
  IconShare,
  IconClockPlay,
} from '@tabler/icons-react';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '@/stores/authStore';
import { cn } from '@/lib/utils';
import { usePermissions } from '@/hooks/usePermissions';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from '@/components/ui/dropdown-menu';
import { Input } from '@/components/ui/input';
import { Switch } from '@/components/ui/switch';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useChannels } from '../context/channels-context';
import { useTestChannel, useUpdateChannel } from '../data/channels';
import { CHANNEL_CONFIGS, getProvider } from '../data/config_channels';
import type { Channel, ChannelPolicies } from '../data/schema';
import { ChannelsStatusDialog } from './channels-status-dialog';

const WEIGHT_PRECISION = 4;
const MIN_WEIGHT = 0;
const MAX_WEIGHT = 100;

const formatWeight = (value: number) => Number(value.toFixed(WEIGHT_PRECISION));
const clampWeight = (value: number) => formatWeight(Math.min(MAX_WEIGHT, Math.max(MIN_WEIGHT, value)));

function isChannelCurrentlyAvailable(policies?: ChannelPolicies | null): boolean {
  if (!policies?.availability?.rules?.length) return true;
  const now = new Date();
  const weekday = now.getDay() === 0 ? 7 : now.getDay();
  const hhmm = now.toTimeString().slice(0, 5);
  let available = true;
  let matched = false;
  for (const rule of policies.availability.rules) {
    if (!rule.enabled) continue;
    if (rule.days?.length && !rule.days.includes(weekday)) continue;
    if (!matchesTimeWindow(hhmm, rule.startTime, rule.endTime)) continue;
    matched = true;
    available = rule.type === 'available';
  }
  if (!matched) {
    // When no rule matches: if any enabled "available" rule exists, treat as whitelist → unavailable
    for (const rule of policies.availability.rules) {
      if (rule.enabled && rule.type === 'available') return false;
    }
    return true;
  }
  return available;
}

function matchesTimeWindow(hhmm: string, start: string, end: string): boolean {
  if (start <= end) return hhmm >= start && hhmm < end;
  return hhmm >= start || hhmm < end;
}

// Status Switch Cell Component to handle status toggle with confirmation dialog
export const StatusSwitchCell = memo(({ row }: { row: Row<Channel> }) => {
  const { t } = useTranslation();
  const { channelPermissions } = usePermissions();
  const { user: authUser } = useAuthStore((state) => state.auth);
  const channel = row.original;
  const [dialogOpen, setDialogOpen] = useState(false);

  const isEnabled = channel.status === 'enabled';
  const isArchived = channel.status === 'archived';
  const isTimeUnavailable = !isChannelCurrentlyAvailable(channel.policies);
  const isOwner = channel.ownerID === authUser?.id;
  const canToggle = channelPermissions.canWrite || (channelPermissions.canManageOwn && isOwner);

  const handleSwitchClick = useCallback(() => {
    if (canToggle && !isArchived) {
      setDialogOpen(true);
    }
  }, [canToggle, isArchived]);

  if (!channelPermissions.canWrite) {
    return <Badge variant='outline'>{channel.status}</Badge>;
  }

  return (
    <div className='flex items-center justify-center'>
      <Switch
        checked={isEnabled}
        onCheckedChange={handleSwitchClick}
        disabled={!canToggle || isArchived}
        data-testid='channel-status-switch'
      />
      {isTimeUnavailable && (
        <Badge variant='outline' className='ml-1 text-xs'>
          {t('channels.columns.timeUnavailable')}
        </Badge>
      )}
      {dialogOpen && <ChannelsStatusDialog open={dialogOpen} onOpenChange={setDialogOpen} currentRow={channel} />}
    </div>
  );
});

StatusSwitchCell.displayName = 'StatusSwitchCell';

// Action Cell Component to handle hooks properly
export const ActionCell = memo(({ row }: { row: Row<Channel> }) => {
  const { t } = useTranslation();
  const channel = row.original;
  const { setOpen, setCurrentRow } = useChannels();
  const { channelPermissions } = usePermissions();
  const testChannel = useTestChannel();
  const isArchived = channel.status === 'archived';
  const hasError = !!channel.errorMessage;
  const hasDisabledAPIKeys = channelPermissions.canWrite && (channel.disabledAPIKeys?.length ?? 0) > 0;

  const handleDefaultTest = async () => {
    try {
      await testChannel.mutateAsync({
        channelID: channel.id,
        modelID: channel.defaultTestModel || undefined,
      });
    } catch (_error) {
      // intentionally ignored
    }
  };

  const handleOpenTestDialog = useCallback(() => {
    setCurrentRow(channel);
    setOpen('test');
  }, [channel, setCurrentRow, setOpen]);

  const handleEdit = useCallback(() => {
    setCurrentRow(channel);
    setOpen('edit');
  }, [channel, setCurrentRow, setOpen]);

  return (
    <div className='flex items-center justify-center gap-1'>
      <Button size='sm' variant='outline' className='h-8 w-8 p-0' onClick={handleEdit}>
        <IconEdit className='h-3 w-3' />
      </Button>
      <Button size='sm' variant='outline' className='h-8 px-3' onClick={handleDefaultTest} disabled={testChannel.isPending}>
        <IconPlayerPlay className='mr-1 h-3 w-3' />
      </Button>
      <DropdownMenu>
        <DropdownMenuTrigger asChild>
          <Button size='sm' variant='outline' className='h-8 w-8 p-0' data-testid='row-actions'>
            <DotsHorizontalIcon className='h-3 w-3' />
          </Button>
        </DropdownMenuTrigger>
        <DropdownMenuContent align='end' className='w-[160px]'>
          <DropdownMenuItem onClick={handleOpenTestDialog}>
            <IconPlayerPlay size={16} className='mr-2' />
            {t('channels.actions.test')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('testHistory');
            }}
          >
            <IconHistory size={16} className='mr-2' />
            {t('channels.actions.testHistory')}
          </DropdownMenuItem>
          <DropdownMenuSeparator />

          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('duplicate');
            }}
          >
            <IconCopy size={16} className='mr-2' />
            {t('common.actions.duplicate')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('modelMapping');
            }}
          >
            <IconRoute size={16} className='mr-2' />
            {t('channels.dialogs.settings.modelMapping.title')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('price');
            }}
          >
            <IconCoin size={16} className='mr-2' />
            {t('channels.actions.modelPrice')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('overrides');
            }}
          >
            <IconAdjustments size={16} className='mr-2' />
            {t('channels.dialogs.settings.overrides.action')}
          </DropdownMenuItem>

          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('proxy');
            }}
          >
            <IconNetwork size={16} className='mr-2' />
            {t('channels.dialogs.proxy.action')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('transformOptions');
            }}
          >
            <IconTransform size={16} className='mr-2' />
            {t('channels.dialogs.transformOptions.action')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('rateLimit');
            }}
          >
            <IconGauge size={16} className='mr-2' />
            {t('channels.dialogs.rateLimit.action')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('endpoints');
            }}
          >
            <IconPlugConnected size={16} className='mr-2' />
            {t('channels.endpoints.title')}
          </DropdownMenuItem>
          {channelPermissions.canWrite && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(channel);
                setOpen('keyManagement');
              }}
            >
              <IconKey size={16} className='mr-2' />
              {t('channels.actions.keyManagement')}
            </DropdownMenuItem>
          )}
          {channelPermissions.canWrite && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(channel);
                setOpen('availability');
              }}
            >
              <IconClockPlay size={16} className='mr-2' />
              {t('channels.dialogs.availability.action')}
            </DropdownMenuItem>
          )}
          {hasDisabledAPIKeys && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(channel);
                setOpen('disabledAPIKeys');
              }}
              className='text-orange-500!'
            >
              <IconKeyOff size={16} className='mr-2' />
              {t('channels.actions.disabledAPIKeys', { count: channel.disabledAPIKeys?.length ?? 0 })}
            </DropdownMenuItem>
          )}
          {hasError && (
            <DropdownMenuItem
              onClick={() => {
                setCurrentRow(channel);
                setOpen('errorResolved');
              }}
              className='text-green-600!'
            >
              <IconCheck size={16} className='mr-2' />
              {t('channels.actions.markErrorResolved')}
            </DropdownMenuItem>
          )}
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('share');
            }}
          >
            <IconShare size={16} className='mr-2' />
            {t('share.dialog.menuItem')}
          </DropdownMenuItem>
          <DropdownMenuSeparator />
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('archive');
            }}
            className={isArchived ? 'text-green-600!' : 'text-orange-500!'}
          >
            {isArchived ? <IconCheck size={16} className='mr-2' /> : <IconArchive size={16} className='mr-2' />}
            {t(isArchived ? 'common.buttons.restore' : 'common.buttons.archive')}
          </DropdownMenuItem>
          <DropdownMenuItem
            onClick={() => {
              setCurrentRow(channel);
              setOpen('delete');
            }}
            className='text-red-500!'
          >
            <IconTrash size={16} className='mr-2' />
            {t('common.buttons.delete')}
          </DropdownMenuItem>
        </DropdownMenuContent>
      </DropdownMenu>
    </div>
  );
});

ActionCell.displayName = 'ActionCell';

export const ExpandCell = ({ row }: { row: any }) => (
  <div className='flex justify-center'>
    <Button
      variant='ghost'
      size='sm'
      className='h-6 w-6 p-0'
      onClick={(e) => {
        e.stopPropagation();
        row.toggleExpanded();
      }}
    >
      {row.getIsExpanded() ? <IconChevronDown className='h-4 w-4' /> : <IconChevronRight className='h-4 w-4' />}
    </Button>
  </div>
);

// ExpandCell.displayName = 'ExpandCell'; // Removed since it's not memoized now, but can keep if desired

function getChannelWebsiteURL(baseURL: string): string | null {
  try {
    const url = new URL(baseURL);
    return url.origin;
  } catch {
    return null;
  }
}

function getProxyURLSummary(proxyURL: string): { label: string; detail?: string } {
  try {
    const url = new URL(proxyURL);
    const pathname = url.pathname === '/' ? '' : url.pathname;
    return {
      label: url.host || proxyURL,
      detail: `${url.protocol}//${url.host}${pathname}`,
    };
  } catch {
    return { label: proxyURL };
  }
}

// Memoized cell components to avoid recreating on every render
export const NameCell = memo(({ row }: { row: Row<Channel> }) => {
  const { t } = useTranslation();
  const channel = row.original;
  const hasError = !!channel.errorMessage;
  const disabledKeysCount = channel.disabledAPIKeys?.length ?? 0;
  const hasDisabledKeys = disabledKeysCount > 0;
  const websiteURL = getChannelWebsiteURL(channel.baseURL);

  const nameElement = websiteURL ? (
    <a
      href={websiteURL}
      target='_blank'
      rel='noopener noreferrer'
      className={cn('truncate font-medium hover:underline', hasError ? 'text-destructive' : '')}
      onClick={(e) => e.stopPropagation()}
    >
      {row.getValue('name')}
    </a>
  ) : (
    <div className={cn('truncate font-medium', hasError && 'text-destructive')}>{row.getValue('name')}</div>
  );

  const content = (
    <div className='flex justify-center'>
      <div className='flex max-w-56 items-center gap-2'>
        {hasError && <IconAlertTriangle className='text-destructive h-4 w-4 shrink-0' />}
        {hasDisabledKeys && <IconKeyOff className='h-4 w-4 shrink-0 text-amber-500' />}
        {nameElement}
      </div>
    </div>
  );

  if (!hasError && !hasDisabledKeys) {
    return content;
  }

  return (
    <Tooltip>
      <TooltipTrigger asChild>{content}</TooltipTrigger>
      <TooltipContent>
        <div className='space-y-1'>
          {hasError && (
            <p className='text-destructive text-sm'>
              {t(`channels.messages.${channel.errorMessage}`, {
                defaultValue: channel.errorMessage,
              })}
            </p>
          )}
          {hasDisabledKeys && (
            <p className='text-sm text-amber-500'>{t('channels.actions.disabledAPIKeys', { count: disabledKeysCount })}</p>
          )}
        </div>
      </TooltipContent>
    </Tooltip>
  );
});

NameCell.displayName = 'NameCell';

export const ProviderCell = memo(({ row }: { row: Row<Channel> }) => {
  const { t } = useTranslation();
  const type = row.original.type;
  const config = CHANNEL_CONFIGS[type];
  const provider = getProvider(type);
  const IconComponent = config.icon;
  return (
    <div className='flex justify-center'>
      <Badge variant='outline' className={cn('capitalize', config.color)}>
        <div className='flex items-center gap-2'>
          <IconComponent size={16} className='shrink-0' />
          <span>{t(`channels.providers.${provider}`)}</span>
        </div>
      </Badge>
    </div>
  );
});

ProviderCell.displayName = 'ProviderCell';

export const TagsCell = memo(({ row }: { row: Row<Channel> }) => {
  const tags = (row.getValue('tags') as string[]) || [];
  if (tags.length === 0) {
    return (
      <div className='flex justify-center'>
        <span className='text-muted-foreground text-xs'>-</span>
      </div>
    );
  }
  return (
    <div className='flex max-w-48 flex-wrap justify-center gap-1'>
      {tags.slice(0, 2).map((tag) => (
        <Badge key={tag} variant='outline' className='text-xs'>
          {tag}
        </Badge>
      ))}
      {tags.length > 2 && (
        <Badge variant='outline' className='text-xs'>
          +{tags.length - 2}
        </Badge>
      )}
    </div>
  );
});

TagsCell.displayName = 'TagsCell';

export const ProxyCell = memo(({ row }: { row: Row<Channel> }) => {
  const { t } = useTranslation();
  const proxy = row.original.settings?.proxy;

  if (!proxy || proxy.type === 'disabled') {
    return (
      <div className='flex justify-center'>
        <span className='text-muted-foreground text-xs'>-</span>
      </div>
    );
  }

  if (proxy.type === 'environment') {
    return (
      <div className='flex justify-center'>
        <span className='text-muted-foreground text-xs'>{t('channels.dialogs.proxy.types.environment')}</span>
      </div>
    );
  }

  const proxyURL = proxy.url?.trim();
  if (!proxyURL) {
    return (
      <div className='flex justify-center'>
        <span className='text-muted-foreground text-xs'>-</span>
      </div>
    );
  }

  const { label, detail } = getProxyURLSummary(proxyURL);
  const content = (
    <div className='flex justify-center'>
      <span className='max-w-40 truncate font-mono text-xs'>{label}</span>
    </div>
  );

  if (detail && detail !== label) {
    return (
      <Tooltip>
        <TooltipTrigger asChild>{content}</TooltipTrigger>
        <TooltipContent>{detail}</TooltipContent>
      </Tooltip>
    );
  }

  return content;
});

ProxyCell.displayName = 'ProxyCell';

export const SupportedModelsCell = memo(({ row }: { row: Row<Channel> }) => {
  const { t } = useTranslation();
  const channel = row.original;
  // Prefer the hide-aware allModelEntries so transformed originals are hidden
  // while untransformed models still display. Fall back to raw supportedModels
  // when allModelEntries is unavailable (e.g. older query without the field).
  const entries = channel.allModelEntries;
  const models =
    entries && entries.length > 0
      ? entries.map((entry) => entry.requestModel)
      : (row.getValue('supportedModels') as string[]);
  const { setOpen, setCurrentRow } = useChannels();

  const handleOpenModelsDialog = useCallback(() => {
    setCurrentRow(channel);
    setOpen('viewModels');
  }, [channel, setCurrentRow, setOpen]);

  return (
    <div className='flex items-center justify-center gap-2'>
      <div className='flex flex-wrap justify-center gap-1 overflow-hidden'>
        {models.slice(0, 5).map((model) => (
          <Badge key={model} variant='secondary' className='block max-w-48 truncate text-left text-xs'>
            {model}
          </Badge>
        ))}
        {models.length > 5 && (
          <Badge
            variant='secondary'
            className='hover:bg-primary hover:text-primary-foreground cursor-pointer text-xs transition-colors'
            onClick={handleOpenModelsDialog}
            title={t('channels.actions.viewModels')}
          >
            +{models.length - 5}
          </Badge>
        )}
      </div>
    </div>
  );
});

SupportedModelsCell.displayName = 'SupportedModelsCell';

export const OrderingWeightCell = memo(({ row }: { row: Row<Channel> }) => {
  const channel = row.original;
  const initialWeight = row.getValue('orderingWeight') as number | null;
  const [isEditing, setIsEditing] = useState(false);
  const [weight, setWeight] = useState<string>(initialWeight?.toString() || '1');
  const updateChannel = useUpdateChannel();
  const { channelPermissions } = usePermissions();
  const inputRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (isEditing && inputRef.current) {
      inputRef.current.focus();
      inputRef.current.select();
    }
  }, [isEditing]);

  const handleDoubleClick = useCallback(() => {
    setIsEditing(true);
    setWeight(initialWeight?.toString() || '1');
  }, [initialWeight]);

  const handleSave = useCallback(async () => {
    const weightValue = clampWeight(Number(weight));
    if (weightValue === initialWeight) {
      setIsEditing(false);
      return;
    }

    try {
      await updateChannel.mutateAsync({
        id: channel.id,
        input: { orderingWeight: weightValue },
      });
      setIsEditing(false);
    } catch (_error) {
      // Error handled by mutation hook
    }
  }, [channel.id, weight, initialWeight, updateChannel]);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      if (e.key === 'Enter') {
        handleSave();
      } else if (e.key === 'Escape') {
        setIsEditing(false);
        setWeight(initialWeight?.toString() || '1');
      }
    },
    [handleSave, initialWeight]
  );

  if (!channelPermissions.canWrite) {
    return <span className={cn('font-mono text-sm', initialWeight == null && 'text-muted-foreground')}>{initialWeight ?? '-'}</span>;
  }

  if (isEditing) {
    return (
      <div className='flex justify-center px-2'>
        <Input
          ref={inputRef}
          type='number'
          inputMode='decimal'
          step='any'
          min={MIN_WEIGHT}
          max={MAX_WEIGHT}
          value={weight}
          onChange={(e) => setWeight(e.target.value)}
          onBlur={handleSave}
          onKeyDown={handleKeyDown}
          className='h-7 w-20 text-center font-mono text-sm'
          disabled={updateChannel.isPending}
        />
      </div>
    );
  }

  return (
    <div className='group flex cursor-pointer items-center justify-center gap-2' onDoubleClick={handleDoubleClick}>
      <span className={cn('font-mono text-sm', initialWeight == null && 'text-muted-foreground')}>{initialWeight ?? '-'}</span>
      {updateChannel.isPending && <IconLoader2 className='text-muted-foreground h-3 w-3 animate-spin' />}
    </div>
  );
});

OrderingWeightCell.displayName = 'OrderingWeightCell';

export const CreatedAtCell = memo(({ row }: { row: Row<Channel> }) => {
  const raw = row.getValue('createdAt') as unknown;
  const date = raw instanceof Date ? raw : new Date(raw as string);

  if (Number.isNaN(date.getTime())) {
    return (
      <div className='flex justify-center'>
        <span className='text-muted-foreground text-xs'>-</span>
      </div>
    );
  }

  return (
    <div className='flex justify-center'>
      <Tooltip>
        <TooltipTrigger asChild>
          <div className='text-muted-foreground cursor-help text-sm'>{format(date, 'yyyy-MM-dd')}</div>
        </TooltipTrigger>
        <TooltipContent>{format(date, 'yyyy-MM-dd HH:mm:ss')}</TooltipContent>
      </Tooltip>
    </div>
  );
});

CreatedAtCell.displayName = 'CreatedAtCell';
