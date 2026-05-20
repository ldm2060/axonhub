import { format, isValid, parseISO } from 'date-fns';
import { Badge } from '@/components/ui/badge';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import type { SharedChannel, SharedModel } from '../data/shared';

function DateCell({ date }: { date: string }) {
  const parsed = parseISO(date);
  if (!isValid(parsed)) return <span>-</span>;
  return (
    <Tooltip>
      <TooltipTrigger className='cursor-help'>
        <span>{format(parsed, 'yyyy-MM-dd')}</span>
      </TooltipTrigger>
      <TooltipContent>{format(parsed, 'yyyy-MM-dd HH:mm:ss')}</TooltipContent>
    </Tooltip>
  );
}

function OwnerCell({ owner }: { owner: SharedChannel['owner'] }) {
  if (!owner) return <span className='text-muted-foreground'>-</span>;
  const name = [owner.firstName, owner.lastName].filter(Boolean).join(' ') || owner.email;
  return <span className='text-sm'>{name}</span>;
}

export interface ColumnDef<T> {
  key: string;
  header: string;
  render: (item: T) => React.ReactNode;
}

export function getChannelColumns(t: (key: string) => string): ColumnDef<SharedChannel>[] {
  return [
    {
      key: 'name',
      header: t('shared.columns.name'),
      render: (channel) => <span className='font-medium'>{channel.name}</span>,
    },
    {
      key: 'type',
      header: t('shared.columns.provider'),
      render: (channel) => (
        <Badge variant='outline' className='capitalize'>
          {channel.type.replace(/_/g, ' ')}
        </Badge>
      ),
    },
    {
      key: 'status',
      header: t('shared.columns.status'),
      render: (channel) => {
        const variant =
          channel.status === 'enabled'
            ? 'default'
            : channel.status === 'disabled'
              ? 'secondary'
              : 'outline';
        return <Badge variant={variant}>{t(`channels.status.${channel.status}`, channel.status)}</Badge>;
      },
    },
    {
      key: 'supportedModels',
      header: t('shared.columns.models'),
      render: (channel) => (
        <Badge variant='secondary'>{channel.supportedModels.length}</Badge>
      ),
    },
    {
      key: 'tags',
      header: t('shared.columns.tags'),
      render: (channel) => {
        if (!channel.tags?.length) return <span className='text-muted-foreground'>-</span>;
        const visible = channel.tags.slice(0, 2);
        const overflow = channel.tags.length - 2;
        return (
          <div className='flex flex-wrap items-center gap-1'>
            {visible.map((tag) => (
              <Badge key={tag} variant='outline' className='text-xs'>
                {tag}
              </Badge>
            ))}
            {overflow > 0 && (
              <Badge variant='outline' className='text-xs'>
                +{overflow}
              </Badge>
            )}
          </div>
        );
      },
    },
    {
      key: 'owner',
      header: t('shared.columns.owner'),
      render: (channel) => <OwnerCell owner={channel.owner} />,
    },
    {
      key: 'createdAt',
      header: t('shared.columns.createdAt'),
      render: (channel) => <DateCell date={channel.createdAt} />,
    },
  ];
}

export function getModelColumns(t: (key: string) => string): ColumnDef<SharedModel>[] {
  return [
    {
      key: 'name',
      header: t('shared.columns.name'),
      render: (model) => <span className='font-medium'>{model.name}</span>,
    },
    {
      key: 'modelID',
      header: t('shared.columns.modelID'),
      render: (model) => <span className='font-mono text-sm'>{model.modelID}</span>,
    },
    {
      key: 'type',
      header: t('shared.columns.type'),
      render: (model) => (
        <Badge variant='secondary'>{model.type}</Badge>
      ),
    },
    {
      key: 'developer',
      header: t('shared.columns.developer'),
      render: (model) => (
        <Badge variant='outline'>{model.developer}</Badge>
      ),
    },
    {
      key: 'group',
      header: t('shared.columns.group'),
      render: (model) => <span className='text-sm'>{model.group}</span>,
    },
    {
      key: 'status',
      header: t('shared.columns.status'),
      render: (model) => {
        const variant =
          model.status === 'enabled'
            ? 'default'
            : model.status === 'disabled'
              ? 'secondary'
              : 'outline';
        return <Badge variant={variant}>{t(`channels.status.${model.status}`, model.status)}</Badge>;
      },
    },
    {
      key: 'owner',
      header: t('shared.columns.owner'),
      render: (model) => <OwnerCell owner={model.owner} />,
    },
    {
      key: 'createdAt',
      header: t('shared.columns.createdAt'),
      render: (model) => <DateCell date={model.createdAt} />,
    },
  ];
}
