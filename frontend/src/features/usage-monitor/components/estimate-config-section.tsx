'use client';

import { useTranslation } from 'react-i18next';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import type { DisplayField } from '../data/schema';

export type EstimateConfigFormValue = {
  fieldKey: string;
  remaining: boolean;
  resetFieldKey: string;
  window: string;
};

// Sentinel for "no selection" — Radix Select forbids empty item values.
const NONE = '__none__';

const ESTIMATE_WINDOWS = [
  { value: '5h', labelKey: 'quota.window.5h' },
  { value: 'daily', labelKey: 'quota.window.daily' },
  { value: 'weekly', labelKey: 'quota.window.weekly' },
  { value: 'monthly', labelKey: 'quota.window.monthly' },
];

type ChannelEdge = { node: { id: string; name: string } };

// EstimateConfigSection is the custom-monitor form block that picks which
// parsed values drive the period quota estimate: the usage ratio field, its
// direction (used vs remaining), the window length, the reset datetime field,
// and the attribution channel whose usage logs price the estimate.
export function EstimateConfigSection({
  displayFields,
  value,
  onChange,
  channels,
  channelId,
  onChannelIdChange,
}: {
  displayFields: DisplayField[];
  value: EstimateConfigFormValue | null;
  onChange: (value: EstimateConfigFormValue | null) => void;
  channels?: ChannelEdge[];
  channelId?: string;
  onChannelIdChange?: (channelId: string) => void;
}) {
  const { t } = useTranslation();

  // Fields that can express a usage ratio: percentages, fractions (computed
  // value/total or raw 0-1), and plain numbers the backend accepts as raw
  // ratios. Reset options come from the datetime-format fields.
  const ratioFields = displayFields.filter((df) => df.format === 'percentage' || df.format === 'fraction' || df.format === 'number');
  const resetFields = displayFields.filter((df) => df.format === 'datetime');

  const patch = (partial: Partial<EstimateConfigFormValue>) => {
    onChange({
      fieldKey: '',
      remaining: false,
      resetFieldKey: '',
      window: '',
      ...(value ?? {}),
      ...partial,
    });
  };

  return (
    <div className='space-y-3'>
      <div className='flex items-center gap-2'>
        <Label className='text-sm font-semibold'>{t('usageMonitor.estimateSection')}</Label>
      </div>
      <p className='text-muted-foreground text-xs'>{t('usageMonitor.estimateSectionHint')}</p>

      {/* Usage ratio field */}
      <div className='space-y-1.5'>
        <Label>{t('usageMonitor.estimateField')}</Label>
        <Select value={value?.fieldKey || NONE} onValueChange={(v) => (v === NONE ? onChange(null) : patch({ fieldKey: v }))}>
          <SelectTrigger className='w-full'>
            <SelectValue placeholder={t('usageMonitor.estimateFieldNone')} />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={NONE}>{t('usageMonitor.estimateFieldNone')}</SelectItem>
            {ratioFields.map((df) => (
              <SelectItem key={df.key} value={df.key}>
                {df.label || df.key}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {value && (
        <>
          {/* Whether the field expresses used or remaining quota */}
          <div className='space-y-1.5'>
            <Label>{t('usageMonitor.estimateDirection')}</Label>
            <Select value={value.remaining ? 'remaining' : 'used'} onValueChange={(v) => patch({ remaining: v === 'remaining' })}>
              <SelectTrigger className='w-full'>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='used'>{t('usageMonitor.estimateDirectionUsed')}</SelectItem>
                <SelectItem value='remaining'>{t('usageMonitor.estimateDirectionRemaining')}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          {/* Window length */}
          <div className='space-y-1.5'>
            <Label>{t('usageMonitor.estimateWindow')}</Label>
            <Select value={value.window || NONE} onValueChange={(v) => patch({ window: v === NONE ? '' : v })}>
              <SelectTrigger className='w-full'>
                <SelectValue placeholder={t('usageMonitor.estimateWindowPlaceholder')} />
              </SelectTrigger>
              <SelectContent>
                {ESTIMATE_WINDOWS.map((w) => (
                  <SelectItem key={w.value} value={w.value}>
                    {t(w.labelKey)}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
            <p className='text-muted-foreground text-xs'>{t('usageMonitor.estimateWindowHint')}</p>
          </div>

          {/* Reset datetime field */}
          <div className='space-y-1.5'>
            <Label>{t('usageMonitor.estimateResetField')}</Label>
            <Select value={value.resetFieldKey || NONE} onValueChange={(v) => patch({ resetFieldKey: v === NONE ? '' : v })}>
              <SelectTrigger className='w-full'>
                <SelectValue placeholder={t('usageMonitor.estimateResetFieldAuto')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value={NONE}>{t('usageMonitor.estimateResetFieldAuto')}</SelectItem>
                {resetFields.map((df) => (
                  <SelectItem key={df.key} value={df.key}>
                    {df.label || df.key}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        </>
      )}

      {/* Attribution channel */}
      {channels && onChannelIdChange && (
        <div className='space-y-1.5'>
          <Label>{t('usageMonitor.estimateChannel')}</Label>
          <Select value={channelId || NONE} onValueChange={(v) => onChannelIdChange(v === NONE ? '' : v)}>
            <SelectTrigger className='w-full'>
              <SelectValue placeholder={t('usageMonitor.estimateChannelNone')} />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={NONE}>{t('usageMonitor.estimateChannelNone')}</SelectItem>
              {channels.map((edge) => (
                <SelectItem key={edge.node.id} value={edge.node.id}>
                  {edge.node.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <p className='text-muted-foreground text-xs'>{t('usageMonitor.estimateChannelHint')}</p>
        </div>
      )}
    </div>
  );
}
