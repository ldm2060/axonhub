import { useTranslation } from 'react-i18next';
import type { ParsedField } from '../data/schema';
import { ParsedFieldDisplay } from './parsed-field-display';

export function MonitorCardField({ field }: { field: ParsedField }) {
  const { t } = useTranslation();

  if (field.error) {
    return (
      <div className="space-y-0.5">
        <div className="text-sm text-muted-foreground">{field.label}</div>
        <div className="text-xs text-red-500">
          {'⚠'} {t('usageMonitor.parseFailed')}: {field.error}
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-0.5">
      <div className="text-sm text-muted-foreground">{field.label}</div>
      <ParsedFieldDisplay field={field} />
    </div>
  );
}
