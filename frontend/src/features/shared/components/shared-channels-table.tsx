import { useTranslation } from 'react-i18next';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table';
import { TableSkeleton } from '@/components/ui/table-skeleton';
import { useMySharedChannels } from '../data/shared';
import { getChannelColumns } from './shared-columns';

export function SharedChannelsTable() {
  const { t } = useTranslation();
  const { data: channels, isLoading } = useMySharedChannels();
  const columns = getChannelColumns(t);

  return (
    <Table>
      <TableHeader>
        <TableRow>
          {columns.map((col) => (
            <TableHead key={col.key}>{col.header}</TableHead>
          ))}
        </TableRow>
      </TableHeader>
      <TableBody>
        {isLoading && <TableSkeleton rows={5} columns={columns.length} />}
        {!isLoading && channels?.length === 0 && (
          <TableRow>
            <TableCell colSpan={columns.length} className='h-24 text-center text-muted-foreground'>
              {t('shared.empty.channels')}
            </TableCell>
          </TableRow>
        )}
        {!isLoading &&
          channels?.map((channel) => (
            <TableRow key={channel.id}>
              {columns.map((col) => (
                <TableCell key={col.key}>{col.render(channel)}</TableCell>
              ))}
            </TableRow>
          ))}
      </TableBody>
    </Table>
  );
}
