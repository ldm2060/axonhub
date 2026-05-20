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
import { useMySharedModels } from '../data/shared';
import { getModelColumns } from './shared-columns';

export function SharedModelsTable() {
  const { t } = useTranslation();
  const { data: models, isLoading } = useMySharedModels();
  const columns = getModelColumns(t);

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
        {!isLoading && models?.length === 0 && (
          <TableRow>
            <TableCell colSpan={columns.length} className='h-24 text-center text-muted-foreground'>
              {t('shared.empty.models')}
            </TableCell>
          </TableRow>
        )}
        {!isLoading &&
          models?.map((model) => (
            <TableRow key={model.id}>
              {columns.map((col) => (
                <TableCell key={col.key}>{col.render(model)}</TableCell>
              ))}
            </TableRow>
          ))}
      </TableBody>
    </Table>
  );
}
