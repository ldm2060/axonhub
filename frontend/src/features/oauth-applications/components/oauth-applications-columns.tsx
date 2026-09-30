import { ColumnDef } from '@tanstack/react-table';
import { TFunction } from 'i18next';
import { Badge } from '@/components/ui/badge';
import { OAuthApplication } from '../data/oauth-applications';
import { OAuthApplicationActions } from './oauth-application-actions';

function formatLastUsed(lastUsedAt?: string | null) {
  if (!lastUsedAt) {
    return <span className='text-muted-foreground'>-</span>;
  }

  return <span className='text-muted-foreground'>{new Date(lastUsedAt).toLocaleString()}</span>;
}

export const createColumns = (t: TFunction): ColumnDef<OAuthApplication>[] => [
  {
    accessorKey: 'name',
    header: t('common.columns.name'),
    cell: ({ row }) => (
      <div className='flex flex-col'>
        <span className='font-medium'>{row.original.name}</span>
        {row.original.description ? <span className='text-muted-foreground text-xs'>{row.original.description}</span> : null}
      </div>
    ),
  },
  {
    accessorKey: 'clientID',
    header: t('oauthApplications.columns.clientId'),
    cell: ({ row }) => <span className='text-muted-foreground font-mono text-sm'>{row.original.clientID}</span>,
  },
  {
    accessorKey: 'redirectUris',
    header: t('oauthApplications.columns.redirectUris'),
    cell: ({ row }) => (
      <div className='flex max-w-md flex-col gap-0.5'>
        {row.original.redirectUris.slice(0, 2).map((uri) => (
          <span key={uri} className='text-muted-foreground truncate font-mono text-xs' title={uri}>
            {uri}
          </span>
        ))}
        {row.original.redirectUris.length > 2 ? (
          <span className='text-muted-foreground text-xs'>+{row.original.redirectUris.length - 2}</span>
        ) : null}
      </div>
    ),
  },
  {
    accessorKey: 'clientType',
    header: t('oauthApplications.columns.clientType'),
    cell: ({ row }) => {
      const clientType = row.original.clientType;
      return (
        <Badge variant='outline'>
          {clientType === 'public' ? t('oauthApplications.clientTypes.public') : t('oauthApplications.clientTypes.confidential')}
        </Badge>
      );
    },
  },
  {
    accessorKey: 'status',
    header: t('common.columns.status'),
    cell: ({ row }) => {
      const status = row.original.status;

      return (
        <Badge variant={status === 'enabled' ? 'default' : 'secondary'}>
          {status === 'enabled' ? t('oauthApplications.status.enabled') : t('oauthApplications.status.disabled')}
        </Badge>
      );
    },
  },
  {
    accessorKey: 'lastUsedAt',
    header: t('oauthApplications.columns.lastUsedAt'),
    cell: ({ row }) => formatLastUsed(row.original.lastUsedAt),
  },
  {
    id: 'actions',
    header: t('common.columns.actions'),
    cell: ({ row }) => <OAuthApplicationActions application={row.original} />,
  },
];
