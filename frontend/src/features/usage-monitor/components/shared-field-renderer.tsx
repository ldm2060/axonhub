import { useTranslation } from 'react-i18next';
import type { ParsedField, DisplayField } from '../data/schema';
import { BadgeDisplay } from './badge-display';

interface SharedFieldRendererProps {
  fields: ParsedField[];
  displayFields?: DisplayField[];
}

/**
 * HSL-based progress bar color calculation.
 * Transitions from green (0%) → yellow (50%) → red (100%).
 */
function getProgressHSL(percentage: number): { h: number; s: number; l: number } {
  const clamped = Math.min(Math.max(percentage || 0, 0), 100);
  const u = clamped / 100;

  let h: number, s: number, l: number;
  if (u < 0.5) {
    const n = u * 2;
    h = 142 - n * (142 - 45);
    s = 71 + n * (93 - 71);
    l = 45 + n * (47 - 45);
  } else {
    const n = (u - 0.5) * 2;
    h = 45 - n * 45;
    s = 93 - n * (93 - 84);
    l = 47 + n * (60 - 47);
  }

  return { h: Math.round(h), s: Math.round(s), l: Math.round(l) };
}

function findDisplayField(displayFields: DisplayField[] | undefined, key: string): DisplayField | undefined {
  return displayFields?.find((df) => df.key === key);
}

function formatRelativeTime(dateStr: string | null, t: (key: string, params?: Record<string, unknown>) => string): string {
  if (!dateStr) return '';
  const resetTimeMs = new Date(dateStr).getTime();
  const diffMs = resetTimeMs - Date.now();

  if (diffMs < 0) return t('quota.label.reset_now');

  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  const d = t('quota.label.d');
  const h = t('quota.label.h');
  const m = t('quota.label.m');

  if (diffDays > 0) return `${diffDays}${d} ${diffHours % 24}${h}`;
  if (diffHours > 0) return `${diffHours}${h} ${diffMins % 60}${m}`;
  return `${diffMins}${m}`;
}

/**
 * Groups percentage/fraction fields with their matching datetime reset fields.
 * Matches by key prefix: "token_pct" pairs with "token_reset", "time_pct" with "time_reset", etc.
 */
function groupFields(fields: ParsedField[]) {
  const groups: { pct: ParsedField; reset?: ParsedField }[] = [];
  const pctFields = fields.filter((f) => f.format === 'percentage' || f.format === 'fraction');
  const datetimeFields = fields.filter((f) => f.format === 'datetime');
  const usedDt = new Set<number>();

  for (const pct of pctFields) {
    const group: { pct: ParsedField; reset?: ParsedField } = { pct };
    // Derive the reset key: "token_pct" → look for "token_reset"
    const prefix = pct.key.replace(/_pct$|_percent$|_usage$/, '');
    const resetKey = `${prefix}_reset`;
    const dtIdx = datetimeFields.findIndex((f, i) => !usedDt.has(i) && (f.key === resetKey || f.key.startsWith(prefix)));
    if (dtIdx >= 0) {
      group.reset = datetimeFields[dtIdx];
      usedDt.add(dtIdx);
    }
    groups.push(group);
  }

  return groups;
}

function ProgressBar({ percentage }: { percentage: number }) {
  const clamped = Math.min(Math.max(percentage || 0, 0), 100);
  const { h, s, l } = getProgressHSL(clamped);
  const bgStyle = { backgroundColor: `hsl(${h}, ${s}%, ${l}%)` };

  return (
    <div className='bg-muted/60 h-1.5 w-full overflow-hidden rounded-full'>
      <div className='h-full transition-all duration-500' style={{ width: `${clamped}%`, ...bgStyle }} />
    </div>
  );
}

/** A single progress-bar field with its reset time embedded in the bottom-right. */
function FieldGroup({
  group,
  t,
}: {
  group: { pct: ParsedField; reset?: ParsedField };
  t: (key: string, params?: Record<string, unknown>) => string;
}) {
  const pct = group.pct.percent ?? 0;
  const resetText = group.reset ? formatRelativeTime(group.reset.value, t) : undefined;

  return (
    <div className='space-y-0.5'>
      <div className='flex items-center justify-between text-xs'>
        <span className='text-muted-foreground font-medium'>{group.pct.label}</span>
        <span className='text-foreground font-medium'>{Math.round(pct)}%</span>
      </div>
      <ProgressBar percentage={pct} />
      {resetText && <div className='text-muted-foreground text-right text-[11px]'>{resetText}</div>}
    </div>
  );
}

function NumberField({ field }: { field: ParsedField }) {
  const valueStr = field.value != null ? Number(field.value).toLocaleString() : '?';
  const unit = field.unit ?? '';

  return (
    <div className='text-xs'>
      <span className='text-muted-foreground font-medium'>{field.label}:</span>{' '}
      <span className='text-foreground font-medium'>
        {valueStr} {unit}
      </span>
    </div>
  );
}

function DatetimeField({ field, t }: { field: ParsedField; t: (key: string, params?: Record<string, unknown>) => string }) {
  if (field.value == null) {
    return <div className='text-muted-foreground text-xs'>--</div>;
  }

  const date = new Date(field.value);
  if (isNaN(date.getTime())) {
    return <div className='text-xs'>{String(field.value)}</div>;
  }

  const now = Date.now();
  const diffMs = date.getTime() - now;
  const absDiffMs = Math.abs(diffMs);
  const isFuture = diffMs > 0;

  let timeText: string;
  if (absDiffMs < 60_000) {
    timeText = t('usageMonitor.relativeTime.justNow');
  } else {
    const absDiffMin = Math.floor(absDiffMs / 60_000);
    const absDiffHour = Math.floor(absDiffMs / 3_600_000);
    const absDiffDay = Math.floor(absDiffMs / 86_400_000);

    if (absDiffMin < 60) {
      timeText = `${absDiffMin}m ${isFuture ? t('usageMonitor.relativeTime.untilReset') : t('usageMonitor.relativeTime.ago')}`;
    } else if (absDiffHour < 24) {
      timeText = `${absDiffHour}h ${isFuture ? t('usageMonitor.relativeTime.untilReset') : t('usageMonitor.relativeTime.ago')}`;
    } else if (absDiffDay < 30) {
      timeText = `${absDiffDay}d ${isFuture ? t('usageMonitor.relativeTime.untilReset') : t('usageMonitor.relativeTime.ago')}`;
    } else {
      timeText = date.toLocaleDateString();
    }
  }

  return (
    <div className='text-xs'>
      <span className='text-muted-foreground font-medium'>{field.label}:</span> <span className='text-foreground'>{timeText}</span>
    </div>
  );
}

function TextField({ field, displayField }: { field: ParsedField; displayField?: DisplayField }) {
  const textValue = String(field.value ?? '--');

  return (
    <div className='text-xs'>
      <span className='text-muted-foreground font-medium'>{field.label}:</span>{' '}
      {displayField?.badge ? (
        <BadgeDisplay value={textValue} badge={displayField.badge} badgePresets={displayField.badgePresets} />
      ) : (
        <span className='text-foreground'>{textValue}</span>
      )}
    </div>
  );
}

/** Render a named group section with a header and grouped field content. */
function NamedGroup({
  groupName,
  groupLabel,
  fields,
  displayFields,
  t,
}: {
  groupName: string;
  groupLabel?: string;
  fields: ParsedField[];
  displayFields?: DisplayField[];
  t: (key: string, params?: Record<string, unknown>) => string;
}) {
  // Within a named group, group percentage/fraction fields with their reset datetime fields
  const fieldGroups = groupFields(fields);
  const groupedKeys = new Set(fieldGroups.map((g) => g.pct.key));
  if (fieldGroups.some((g) => g.reset)) {
    fieldGroups.forEach((g) => {
      if (g.reset) groupedKeys.add(g.reset.key);
    });
  }

  const otherFields = fields.filter((f) => !groupedKeys.has(f.key));

  const headerText = groupLabel || groupName;

  return (
    <div className='border-border/60 space-y-2 border-t border-dashed pt-2.5'>
      <div className='text-muted-foreground text-xs font-medium tracking-wide uppercase'>{headerText}</div>
      <div className='space-y-2'>
        {fieldGroups.length > 0 && (
          <div className='space-y-2'>
            {fieldGroups.map((group) => (
              <FieldGroup key={group.pct.key} group={group} t={t} />
            ))}
          </div>
        )}
        {otherFields.length > 0 && (
          <div className='space-y-1'>
            {otherFields.map((field) => {
              const displayField = findDisplayField(displayFields, field.key);
              // Skip fields whose value is null (variable not extracted, e.g. additional_1 on Pro Lite)
              if (field.value === null && !field.error) return null;
              if (field.error) {
                return (
                  <div key={field.key} className='text-xs text-red-500'>
                    ⚠ {t('usageMonitor.parseFailed')}: {field.error}
                  </div>
                );
              }
              switch (field.format) {
                case 'number':
                  return <NumberField key={field.key} field={field} />;
                case 'datetime':
                  return <DatetimeField key={field.key} field={field} t={t} />;
                case 'text':
                default:
                  return <TextField key={field.key} field={field} displayField={displayField} />;
              }
            })}
          </div>
        )}
      </div>
    </div>
  );
}

export function SharedFieldRenderer({ fields, displayFields }: SharedFieldRendererProps) {
  const { t } = useTranslation();

  if (!fields || fields.length === 0) return null;

  // Partition fields into: ungrouped, and named groups
  const ungroupedFields: ParsedField[] = [];
  const groupMap = new Map<string, { fields: ParsedField[]; groupLabel?: string }>();

  // Maintain group insertion order
  const groupOrder: string[] = [];

  for (const field of fields) {
    if (field.group) {
      if (!groupMap.has(field.group)) {
        groupMap.set(field.group, { fields: [], groupLabel: field.groupLabel });
        groupOrder.push(field.group);
      }
      groupMap.get(field.group)!.fields.push(field);
      // Update groupLabel if we find a better one (from a field with groupLabel set)
      if (field.groupLabel && !groupMap.get(field.group)!.groupLabel) {
        groupMap.get(field.group)!.groupLabel = field.groupLabel;
      }
    } else {
      ungroupedFields.push(field);
    }
  }

  // Within ungrouped fields, auto-group percentage/fraction fields with their reset datetime fields
  const autoGroups = groupFields(ungroupedFields);
  const autoGroupedKeys = new Set(autoGroups.map((g) => g.pct.key));
  if (autoGroups.some((g) => g.reset)) {
    autoGroups.forEach((g) => {
      if (g.reset) autoGroupedKeys.add(g.reset.key);
    });
  }
  const ungroupedOther = ungroupedFields.filter((f) => !autoGroupedKeys.has(f.key));

  return (
    <div className='space-y-2'>
      {/* Auto-grouped progress bars (ungrouped percentage/fraction + datetime reset pairs) */}
      {autoGroups.length > 0 && (
        <div className='space-y-2'>
          {autoGroups.map((group) => (
            <FieldGroup key={group.pct.key} group={group} t={t} />
          ))}
        </div>
      )}

      {/* Ungrouped other fields (text, number, datetime not used as reset, badges) */}
      {ungroupedOther.length > 0 && (
        <div className='space-y-1'>
          {ungroupedOther.map((field) => {
            const displayField = findDisplayField(displayFields, field.key);
            // Skip fields whose value is null (variable not extracted)
            if (field.value === null && !field.error) return null;
            if (field.error) {
              return (
                <div key={field.key} className='text-xs text-red-500'>
                  ⚠ {t('usageMonitor.parseFailed')}: {field.error}
                </div>
              );
            }
            switch (field.format) {
              case 'number':
                return <NumberField key={field.key} field={field} />;
              case 'datetime':
                return <DatetimeField key={field.key} field={field} t={t} />;
              case 'text':
              default:
                return <TextField key={field.key} field={field} displayField={displayField} />;
            }
          })}
        </div>
      )}

      {/* Named groups */}
      {groupOrder.map((groupName) => {
        const groupData = groupMap.get(groupName)!;
        // If all fields in the group have null values (e.g. additional_1 on Pro Lite), skip the entire group
        const hasAnyValue = groupData.fields.some((f) => f.value !== null);
        if (!hasAnyValue) return null;

        return (
          <NamedGroup
            key={groupName}
            groupName={groupName}
            groupLabel={groupData.groupLabel}
            fields={groupData.fields}
            displayFields={displayFields}
            t={t}
          />
        );
      })}
    </div>
  );
}
