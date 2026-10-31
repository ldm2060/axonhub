'use client';

import { Plus, X } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectGroup, SelectItem, SelectLabel, SelectTrigger, SelectValue } from '@/components/ui/select';
import type { DisplayField, Variable } from '../data/schema';

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

// EstimateConfigSection edits the monitor's period quota estimate entries —
// one row per window, since providers commonly expose a 5h rolling window and
// a longer pool at the same time. Each row picks the usage ratio field (and
// its direction); a row with no usage field (template monitors) stamps the
// window onto the template's own limits without changing their ratios. The
// attribution channel select is rendered when the parent passes channel
// props.
export function EstimateConfigSection({
  displayFields,
  variables,
  values,
  onChange,
  channels,
  channelId,
  onChannelIdChange,
}: {
  displayFields: DisplayField[];
  variables?: Variable[];
  values: EstimateConfigFormValue[];
  onChange: (values: EstimateConfigFormValue[]) => void;
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

  const addRow = () => onChange([...values, { fieldKey: '', remaining: false, resetFieldKey: '', window: '' }]);
  const removeRow = (index: number) => onChange(values.filter((_, i) => i !== index));
  const patchRow = (index: number, partial: Partial<EstimateConfigFormValue>) =>
    onChange(values.map((row, i) => (i === index ? { ...row, ...partial } : row)));

  return (
    <div className='space-y-3'>
      <div className='flex items-center gap-2'>
        <Label className='text-sm font-semibold'>{t('usageMonitor.estimateSection')}</Label>
      </div>
      <p className='text-muted-foreground text-xs'>{t('usageMonitor.estimateSectionHint')}</p>

      {values.map((row, index) => (
        <div key={index} className='border-border/60 relative space-y-2 rounded-md border p-2.5 pr-8'>
          <button
            type='button'
            onClick={() => removeRow(index)}
            className='text-muted-foreground hover:text-foreground absolute top-2 right-2'
            aria-label={t('usageMonitor.estimateRemove')}
          >
            <X className='size-3.5' />
          </button>

          <div className='grid grid-cols-2 gap-2'>
            <div className='space-y-1'>
              <Label className='text-muted-foreground text-[11px]'>{t('usageMonitor.estimateField')}</Label>
              <Select value={row.fieldKey || undefined} onValueChange={(v) => patchRow(index, { fieldKey: v })}>
                <SelectTrigger className='h-8 w-full'>
                  <SelectValue placeholder={t('usageMonitor.estimateFieldPlaceholder')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectGroup>
                    <SelectLabel>{t('usageMonitor.displayFieldSection')}</SelectLabel>
                    {ratioFields.map((df) => (
                      <SelectItem key={df.key} value={df.key}>
                        {df.label || df.key}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                  {(variables?.length ?? 0) > 0 && (
                    <SelectGroup>
                      <SelectLabel>{t('usageMonitor.variableSection')}</SelectLabel>
                      {variables!.map((v) => (
                        <SelectItem key={v.key} value={`var:${v.key}`}>
                          {v.key}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  )}
                </SelectContent>
              </Select>
            </div>
            <div className='space-y-1'>
              <Label className='text-muted-foreground text-[11px]'>{t('usageMonitor.estimateWindow')}</Label>
              <Select value={row.window || undefined} onValueChange={(v) => patchRow(index, { window: v })}>
                <SelectTrigger className='h-8 w-full'>
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
            </div>
          </div>

          <div className='grid grid-cols-2 gap-2'>
            <div className='space-y-1'>
              <Label className='text-muted-foreground text-[11px]'>{t('usageMonitor.estimateDirection')}</Label>
              <Select value={row.remaining ? 'remaining' : 'used'} onValueChange={(v) => patchRow(index, { remaining: v === 'remaining' })}>
                <SelectTrigger className='h-8 w-full'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value='used'>{t('usageMonitor.estimateDirectionUsed')}</SelectItem>
                  <SelectItem value='remaining'>{t('usageMonitor.estimateDirectionRemaining')}</SelectItem>
                </SelectContent>
              </Select>
            </div>
            <div className='space-y-1'>
              <Label className='text-muted-foreground text-[11px]'>{t('usageMonitor.estimateResetField')}</Label>
              <Select value={row.resetFieldKey || undefined} onValueChange={(v) => patchRow(index, { resetFieldKey: v === NONE ? '' : v })}>
                <SelectTrigger className='h-8 w-full'>
                  <SelectValue placeholder={t('usageMonitor.estimateResetFieldAuto')} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value={NONE}>{t('usageMonitor.estimateResetFieldAuto')}</SelectItem>
                  <SelectGroup>
                    <SelectLabel>{t('usageMonitor.displayFieldSection')}</SelectLabel>
                    {resetFields.map((df) => (
                      <SelectItem key={df.key} value={df.key}>
                        {df.label || df.key}
                      </SelectItem>
                    ))}
                  </SelectGroup>
                  {(variables?.length ?? 0) > 0 && (
                    <SelectGroup>
                      <SelectLabel>{t('usageMonitor.variableSection')}</SelectLabel>
                      {variables!.map((v) => (
                        <SelectItem key={v.key} value={`var:${v.key}`}>
                          {v.key}
                        </SelectItem>
                      ))}
                    </SelectGroup>
                  )}
                </SelectContent>
              </Select>
            </div>
          </div>
        </div>
      ))}

      <Button type='button' variant='outline' size='sm' onClick={addRow} className='w-full'>
        <Plus className='mr-1 size-3.5' />
        {t('usageMonitor.estimateAddWindow')}
      </Button>

      {/* Attribution channel */}
      {channels && onChannelIdChange && (
        <div className='space-y-1.5'>
          <Label>{t('usageMonitor.estimateChannel')}</Label>
          <Select value={channelId || undefined} onValueChange={(v) => onChannelIdChange(v === NONE ? '' : v)}>
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
