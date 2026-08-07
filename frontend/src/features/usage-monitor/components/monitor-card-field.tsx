import { useTranslation } from 'react-i18next';
import type { ParsedField, DisplayField } from '../data/schema';
import { ParsedFieldDisplay } from './parsed-field-display';

interface MonitorCardFieldProps {
  field: ParsedField;
  displayFields?: DisplayField[];
}

export function MonitorCardField({ field, displayFields }: MonitorCardFieldProps) {
  const { t } = useTranslation();

  if (field.error) {
    return (
      <div className='space-y-0.5'>
        <div className='text-muted-foreground text-sm'>{field.label}</div>
        <div className='text-xs text-red-500'>
          {'⚠'} {t('usageMonitor.parseFailed')}: {field.error}
        </div>
      </div>
    );
  }

  return (
    <div className='space-y-0.5'>
      <div className='text-muted-foreground text-sm'>{field.label}</div>
      <ParsedFieldDisplay field={field} displayFields={displayFields} />
    </div>
  );
}
