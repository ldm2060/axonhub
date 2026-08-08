import { useTranslation } from 'react-i18next';
import type { ParsedField, DisplayField } from '../data/schema';
import { BadgeDisplay } from './badge-display';

function formatCompactNumber(value: number): string {
  if (value >= 1_000_000_000) return `${(value / 1_000_000_000).toFixed(1).replace(/\.0$/, '')}B`;
  if (value >= 1_000_000) return `${(value / 1_000_000).toFixed(1).replace(/\.0$/, '')}M`;
  if (value >= 1_000) return `${(value / 1_000).toFixed(1).replace(/\.0$/, '')}K`;
  return value.toLocaleString();
}

function getProgressColor(percent: number): string {
  if (percent > 80) return 'bg-red-500';
  if (percent > 60) return 'bg-yellow-500';
  return 'bg-green-500';
}

function findDisplayField(displayFields: DisplayField[] | undefined, key: string): DisplayField | undefined {
  return displayFields?.find((df) => df.key === key);
}

function PercentageDisplay({ field, badgeMeta: _badgeMeta }: { field: ParsedField; badgeMeta?: DisplayField }) {
  const pct = field.percent ?? 0;
  const clamped = Math.min(Math.max(pct, 0), 100);
  const valueStr = field.value != null ? formatCompactNumber(Number(field.value)) : null;
  const totalStr = field.total != null ? formatCompactNumber(Number(field.total)) : null;
  const unit = field.unit ?? '';

  const detailLine =
    valueStr != null && totalStr != null
      ? `${valueStr} / ${totalStr} ${unit}`
      : valueStr != null
        ? `${valueStr} ${unit}`
        : `${Math.round(clamped)}% ${unit}`;

  return (
    <div className='space-y-1'>
      <div className='bg-muted/60 h-2 w-full overflow-hidden rounded-full'>
        <div className={`h-full rounded-full transition-all duration-500 ${getProgressColor(clamped)}`} style={{ width: `${clamped}%` }} />
      </div>
      <div className='text-muted-foreground text-xs'>{detailLine}</div>
    </div>
  );
}

function FractionDisplay({ field, badgeMeta: _badgeMeta }: { field: ParsedField; badgeMeta?: DisplayField }) {
  const valueStr = field.value != null ? Number(field.value).toLocaleString() : '--';
  const totalStr = field.total != null ? Number(field.total).toLocaleString() : null;
  const unit = field.unit ?? '';

  const detailLine = totalStr != null ? `${valueStr} / ${totalStr} ${unit}` : `${valueStr} ${unit}`;

  return <div className='text-sm'>{detailLine}</div>;
}

function NumberDisplay({ field, badgeMeta: _badgeMeta }: { field: ParsedField; badgeMeta?: DisplayField }) {
  const valueStr = field.value != null ? Number(field.value).toLocaleString() : '?';
  const unit = field.unit ?? '';

  return (
    <div className='text-sm'>
      {valueStr} {unit}
    </div>
  );
}

function DatetimeDisplay({ field, badgeMeta: _badgeMeta }: { field: ParsedField; badgeMeta?: DisplayField }) {
  const { t } = useTranslation();

  if (field.value == null) {
    return <div className='text-muted-foreground text-sm'>--</div>;
  }

  const date = new Date(field.value);
  if (isNaN(date.getTime())) {
    return <div className='text-sm'>{String(field.value)}</div>;
  }

  const now = Date.now();
  const diffMs = date.getTime() - now;
  const absDiffMs = Math.abs(diffMs);
  const isFuture = diffMs > 0;

  if (absDiffMs < 60_000) {
    return <div className='text-sm'>{isFuture ? t('usageMonitor.relativeTime.justNow') : t('usageMonitor.relativeTime.justNow')}</div>;
  }

  const absDiffMin = Math.floor(absDiffMs / 60_000);
  const absDiffHour = Math.floor(absDiffMs / 3_600_000);
  const absDiffDay = Math.floor(absDiffMs / 86_400_000);

  if (absDiffMin < 60) {
    return (
      <div className='text-sm'>
        {absDiffMin}m {isFuture ? t('usageMonitor.relativeTime.untilReset') : t('usageMonitor.relativeTime.ago')}
      </div>
    );
  }

  if (absDiffHour < 24) {
    return (
      <div className='text-sm'>
        {absDiffHour}h {isFuture ? t('usageMonitor.relativeTime.untilReset') : t('usageMonitor.relativeTime.ago')}
      </div>
    );
  }

  if (absDiffDay < 30) {
    return (
      <div className='text-sm'>
        {absDiffDay}d {isFuture ? t('usageMonitor.relativeTime.untilReset') : t('usageMonitor.relativeTime.ago')}
      </div>
    );
  }

  return <div className='text-sm'>{date.toLocaleDateString()}</div>;
}

function TextDisplay({ field, badgeMeta }: { field: ParsedField; badgeMeta?: DisplayField }) {
  const textValue = String(field.value ?? '--');

  if (badgeMeta?.badge) {
    return (
      <div className='text-sm'>
        <BadgeDisplay value={textValue} badge={badgeMeta.badge} badgePresets={badgeMeta.badgePresets ?? undefined} />
      </div>
    );
  }

  return <div className='text-sm'>{textValue}</div>;
}

interface ParsedFieldDisplayProps {
  field: ParsedField;
  displayFields?: DisplayField[];
}

export function ParsedFieldDisplay({ field, displayFields }: ParsedFieldDisplayProps) {
  const badgeMeta = findDisplayField(displayFields, field.key);

  switch (field.format) {
    case 'percentage':
      return <PercentageDisplay field={field} badgeMeta={badgeMeta} />;
    case 'fraction':
      return <FractionDisplay field={field} badgeMeta={badgeMeta} />;
    case 'number':
      return <NumberDisplay field={field} badgeMeta={badgeMeta} />;
    case 'datetime':
      return <DatetimeDisplay field={field} badgeMeta={badgeMeta} />;
    case 'text':
    default:
      return <TextDisplay field={field} badgeMeta={badgeMeta} />;
  }
}
