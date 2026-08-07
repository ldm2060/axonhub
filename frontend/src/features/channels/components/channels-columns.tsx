import { ColumnDef, Row, Table } from '@tanstack/react-table';
import { useTranslation } from 'react-i18next';
import { Checkbox } from '@/components/ui/checkbox';
import { DataTableColumnHeader } from '@/components/data-table-column-header';
import { Channel } from '../data/schema';
import { ChannelHealthCell } from './channel-health-cell';
import { ChannelLimiterCell } from './channel-limiter-cell';
import {
  ActionCell,
  CreatedAtCell,
  ExpandCell,
  NameCell,
  OrderingWeightCell,
  ProviderCell,
  ProxyCell,
  StatusSwitchCell,
  SupportedModelsCell,
  TagsCell,
} from './channels-column-cells';

interface CreateColumnsOptions {
  hideOrderingWeight?: boolean;
}

export const createColumns = (
  t: ReturnType<typeof useTranslation>['t'],
  canWrite: boolean = true,
  options?: CreateColumnsOptions
): ColumnDef<Channel>[] => {
  return [
    {
      id: 'expand',
      header: () => null,
      meta: {
        className: 'w-8 min-w-8 text-center',
      },
      cell: ExpandCell,
      enableSorting: false,
      enableHiding: false,
    },
    ...(canWrite
      ? [
          {
            id: 'select',
            header: ({ table }: { table: Table<Channel> }) => (
              <div className='flex justify-center'>
                <Checkbox
                  checked={table.getIsAllPageRowsSelected() || (table.getIsSomePageRowsSelected() && 'indeterminate')}
                  onCheckedChange={(value) => table.toggleAllPageRowsSelected(!!value)}
                  aria-label={t('common.columns.selectAll')}
                  className='translate-y-[2px]'
                />
              </div>
            ),
            cell: ({ row }: { row: Row<Channel> }) => (
              <div className='flex justify-center'>
                <Checkbox
                  checked={row.getIsSelected()}
                  onCheckedChange={(value) => row.toggleSelected(!!value)}
                  aria-label={t('common.columns.selectRow')}
                  className='translate-y-[2px]'
                />
              </div>
            ),
            meta: {
              className: 'text-center',
            },
            enableSorting: false,
            enableHiding: false,
          },
        ]
      : []),
    {
      accessorKey: 'name',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.name')} className='justify-center' />,
      cell: NameCell,
      meta: {
        className: 'md:table-cell min-w-48 text-center',
      },
      enableHiding: false,
      enableSorting: true,
    },
    {
      id: 'provider',
      accessorKey: 'type',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.provider')} className='justify-center' />,
      cell: ProviderCell,
      meta: {
        className: 'text-center',
      },
      filterFn: (row, _id, value) => {
        return value.includes(row.original.type);
      },
      enableSorting: true,
      enableHiding: false,
    },
    {
      accessorKey: 'status',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.status')} className='justify-center' />,
      cell: StatusSwitchCell,
      meta: {
        className: 'text-center',
      },
      enableSorting: true,
      enableHiding: false,
    },

    {
      accessorKey: 'tags',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.tags')} className='justify-center' />,
      cell: TagsCell,
      meta: {
        className: 'text-center',
      },
      filterFn: (row, id, value) => {
        const tags = (row.getValue(id) as string[]) || [];
        // Single select: value is a string, not an array
        return tags.includes(value as string);
      },
      enableSorting: false,
      enableHiding: true,
    },
    {
      id: 'model',
      accessorFn: () => '', // Virtual column for filtering only
      header: () => null,
      cell: () => null,
      filterFn: () => true, // Server-side filtering, always return true
      enableSorting: false,
      enableHiding: true,
      enableColumnFilter: false,
      enableGlobalFilter: false,
    },
    {
      accessorKey: 'supportedModels',
      header: ({ column }) => (
        <DataTableColumnHeader column={column} title={t('channels.columns.supportedModels')} className='justify-center' />
      ),
      cell: SupportedModelsCell,
      meta: {
        className: 'max-w-64 text-center',
      },
      enableSorting: false,
    },
    {
      id: 'proxy',
      accessorFn: (row) => row.settings?.proxy?.url ?? row.settings?.proxy?.type ?? '',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.proxy')} className='justify-center' />,
      cell: ProxyCell,
      meta: {
        className: 'w-32 min-w-32 text-center',
      },
      enableSorting: false,
      enableHiding: true,
    },
    {
      id: 'health',
      accessorKey: 'health',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('channels.columns.health')} className='justify-center' />,
      cell: ({ row }: { row: Row<Channel> }) => {
        const probePoints = (row.original as any).probePoints || [];
        const limiterStats = row.original.liveLimiterStats;
        return (
          <div className='flex flex-col items-center gap-1'>
            <ChannelHealthCell points={probePoints} />
            {limiterStats ? <ChannelLimiterCell stats={limiterStats} /> : null}
          </div>
        );
      },
      meta: {
        className: 'text-center',
      },
      enableSorting: false,
      enableHiding: true,
    },
    ...(!options?.hideOrderingWeight
      ? [
          {
            accessorKey: 'orderingWeight',
            header: ({ column }: { column: any }) => (
              <DataTableColumnHeader column={column} title={t('channels.columns.orderingWeight')} className='justify-center' />
            ),
            cell: OrderingWeightCell,
            meta: {
              className: 'w-20 min-w-20 text-center',
            },
            sortingFn: 'alphanumeric',
            enableSorting: true,
            enableHiding: true,
          },
        ]
      : []),
    {
      accessorKey: 'createdAt',
      header: ({ column }) => <DataTableColumnHeader column={column} title={t('common.columns.createdAt')} className='justify-center' />,
      cell: CreatedAtCell,
      meta: {
        className: 'text-center',
      },
      enableSorting: true,
      enableHiding: false,
    },
    ...(canWrite
      ? [
          {
            id: 'action',
            header: ({ column }: { column: any }) => (
              <DataTableColumnHeader column={column} title={t('common.columns.actions')} className='justify-center' />
            ),
            cell: ActionCell,
            meta: {
              className: 'text-center',
            },
            enableSorting: false,
            enableHiding: false,
          },
        ]
      : []),
  ];
};
