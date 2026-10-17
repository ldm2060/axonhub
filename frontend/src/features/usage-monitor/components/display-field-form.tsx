'use client';

import { memo, useCallback, useMemo, useState, type Dispatch, type SetStateAction } from 'react';
import { Plus, Trash2, ChevronDown } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import type { DisplayField, Variable } from '../data/schema';

interface Props {
  displayFields: DisplayField[];
  variables: Variable[];
  onChange: Dispatch<SetStateAction<DisplayField[]>>;
  readOnly?: boolean;
}

function generateKey(): string {
  return `df_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

// Memoized so editing one item (or unrelated dialog state) doesn't re-render every item.
// `variableKeys` is passed as a joined string (not the array) so its identity stays
// value-stable while the user edits variable paths — array identity would break the memo.
interface DisplayFieldItemProps {
  df: DisplayField;
  index: number;
  variableKeys: string;
  readOnly?: boolean;
  onUpdate: (index: number, updates: Partial<DisplayField>) => void;
  onRemove: (index: number) => void;
}

function DisplayFieldItem({ df, index, variableKeys, readOnly, onUpdate, onRemove }: DisplayFieldItemProps) {
  const { t } = useTranslation();
  const [badgeOpen, setBadgeOpen] = useState(false);

  const keys = useMemo(() => (variableKeys ? variableKeys.split('\n') : []), [variableKeys]);

  // Determine if valueRef matches a variable key
  const isVariableRef = keys.includes(df.valueRef);

  return (
    <div className='space-y-3 rounded-lg border p-3'>
      <div className='flex items-center justify-between'>
        <span className='text-sm font-medium'>
          {t('usageMonitor.displayField.label')} #{index + 1}
        </span>
        {!readOnly && (
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            onClick={() => onRemove(index)}
            className='text-destructive hover:text-destructive'
            aria-label={t('usageMonitor.deleteDisplayField')}
          >
            <Trash2 className='size-4' />
          </Button>
        )}
      </div>

      <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
        {/* Display Label */}
        <div className='space-y-1.5'>
          <Label>{t('usageMonitor.field.label')}</Label>
          <Input
            value={df.label}
            onChange={(e) => onUpdate(index, { label: e.target.value })}
            placeholder={t('usageMonitor.field.label')}
            disabled={readOnly}
          />
        </div>

        {/* Display Format */}
        <div className='space-y-1.5'>
          <Label>{t('usageMonitor.field.format')}</Label>
          <Select value={df.format} onValueChange={(v) => onUpdate(index, { format: v as DisplayField['format'] })} disabled={readOnly}>
            <SelectTrigger className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='percentage'>{t('usageMonitor.format.percentage')}</SelectItem>
              <SelectItem value='fraction'>{t('usageMonitor.format.fraction')}</SelectItem>
              <SelectItem value='number'>{t('usageMonitor.format.number')}</SelectItem>
              <SelectItem value='datetime'>{t('usageMonitor.format.datetime')}</SelectItem>
              <SelectItem value='text'>{t('usageMonitor.format.text')}</SelectItem>
            </SelectContent>
          </Select>
        </div>

        {/* Value Ref - variable selector */}
        <div className='space-y-1.5'>
          <Label>{t('usageMonitor.displayField.valueRef')}</Label>
          <Select
            value={isVariableRef ? df.valueRef : '__expression__'}
            onValueChange={(val) => {
              if (val === '__expression__') {
                // Switch to expression mode - clear valueRef so user types expression
                onUpdate(index, { valueRef: '' });
              } else {
                onUpdate(index, { valueRef: val });
              }
            }}
            disabled={readOnly}
          >
            <SelectTrigger className='w-full'>
              <SelectValue placeholder={t('usageMonitor.displayField.valueRef')} />
            </SelectTrigger>
            <SelectContent>
              {keys.map((k) => (
                <SelectItem key={k} value={k}>
                  {k}
                </SelectItem>
              ))}
              <SelectItem value='__expression__'>{t('usageMonitor.displayField.expression')}</SelectItem>
            </SelectContent>
          </Select>
        </div>

        {/* Expression Input - shown when valueRef is not a variable key */}
        {!isVariableRef && (
          <div className='space-y-1.5'>
            <Label>{t('usageMonitor.field.expression')}</Label>
            <Input
              value={df.valueRef}
              onChange={(e) => onUpdate(index, { valueRef: e.target.value })}
              placeholder='${var1}/${var2}*100'
              className='font-mono'
              disabled={readOnly}
            />
            <p className='text-muted-foreground text-xs'>{t('usageMonitor.field.expressionHint')}</p>
          </div>
        )}

        {/* Unit */}
        <div className='space-y-1.5'>
          <Label>{t('usageMonitor.field.unit')}</Label>
          <Input
            value={df.unit ?? ''}
            onChange={(e) => onUpdate(index, { unit: e.target.value || undefined })}
            placeholder='%'
            disabled={readOnly}
          />
        </div>

        {/* Group */}
        <div className='space-y-1.5'>
          <Label>{t('usageMonitor.displayField.group')}</Label>
          <Input
            value={df.group ?? ''}
            onChange={(e) => onUpdate(index, { group: e.target.value || undefined })}
            placeholder='primary'
            disabled={readOnly}
          />
        </div>

        {/* Group Label Ref */}
        <div className='space-y-1.5'>
          <Label>{t('usageMonitor.displayField.groupLabelRef')}</Label>
          <Input
            value={df.groupLabelRef ?? ''}
            onChange={(e) => onUpdate(index, { groupLabelRef: e.target.value || undefined })}
            placeholder='additional_0_name'
            className='font-mono'
            disabled={readOnly}
          />
        </div>

        {/* Total Ref - only for fraction format */}
        {df.format === 'fraction' && (
          <div className='space-y-1.5'>
            <Label>{t('usageMonitor.displayField.totalRef')}</Label>
            <Select
              value={df.totalRef || '__none__'}
              onValueChange={(val) => onUpdate(index, { totalRef: val === '__none__' ? undefined : val })}
              disabled={readOnly}
            >
              <SelectTrigger className='w-full'>
                <SelectValue placeholder={t('usageMonitor.displayField.totalRef')} />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='__none__'>{t('usageMonitor.displayField.none')}</SelectItem>
                {keys.map((k) => (
                  <SelectItem key={k} value={k}>
                    {k}
                  </SelectItem>
                ))}
              </SelectContent>
            </Select>
          </div>
        )}
      </div>

      {/* Badge Config - collapsible */}
      <Collapsible open={badgeOpen} onOpenChange={setBadgeOpen}>
        <CollapsibleTrigger asChild>
          <Button
            type='button'
            variant='ghost'
            size='sm'
            className='text-muted-foreground hover:text-foreground w-full justify-start px-0 text-xs'
          >
            <ChevronDown className={`mr-1 size-3 transition-transform ${badgeOpen ? 'rotate-0' : '-rotate-90'}`} />
            {t('usageMonitor.displayField.badgeConfig')}
          </Button>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <div className='grid grid-cols-1 gap-3 pt-2 sm:grid-cols-2'>
            <div className='space-y-1.5'>
              <Label>{t('usageMonitor.displayField.badgeKey')}</Label>
              <Input
                value={df.badge ?? ''}
                onChange={(e) => onUpdate(index, { badge: e.target.value || undefined })}
                placeholder='tier'
                className='font-mono'
                disabled={readOnly}
              />
            </div>
            <div className='space-y-1.5'>
              <Label>{t('usageMonitor.displayField.badgePresets')}</Label>
              <Input
                value={df.badgePresets ?? ''}
                onChange={(e) => onUpdate(index, { badgePresets: e.target.value || undefined })}
                placeholder='{"free":"gradient","pro":"blue"}'
                className='font-mono text-xs'
                disabled={readOnly}
              />
            </div>
          </div>
        </CollapsibleContent>
      </Collapsible>
    </div>
  );
}

const MemoizedDisplayFieldItem = memo(DisplayFieldItem);

// Memoized so typing in unrelated dialog fields (name, API key, ...) skips
// re-rendering the whole display-field list, which is large for template channels.
export const DisplayFieldForm = memo(function DisplayFieldForm({ displayFields, variables, onChange, readOnly }: Props) {
  const { t } = useTranslation();

  const updateField = useCallback(
    (index: number, updates: Partial<DisplayField>) => {
      onChange((prev) => {
        const next = [...prev];
        next[index] = { ...next[index], ...updates };
        return next;
      });
    },
    [onChange]
  );

  const removeField = useCallback((index: number) => onChange((prev) => prev.filter((_, i) => i !== index)), [onChange]);

  const addField = useCallback(
    () =>
      onChange((prev) => [
        ...prev,
        {
          key: generateKey(),
          label: '',
          valueRef: '',
          format: 'text',
          displayOrder: prev.length,
        },
      ]),
    [onChange]
  );

  // Joined string keeps this prop value-stable across variable edits (see DisplayFieldItemProps).
  const variableKeys = useMemo(() => variables.map((v) => v.key).join('\n'), [variables]);

  return (
    <div className='space-y-4'>
      {displayFields.map((df, index) => (
        <MemoizedDisplayFieldItem
          key={df.key}
          df={df}
          index={index}
          variableKeys={variableKeys}
          readOnly={readOnly}
          onUpdate={updateField}
          onRemove={removeField}
        />
      ))}

      {!readOnly && (
        <Button type='button' variant='outline' onClick={addField} className='w-full'>
          <Plus className='mr-1.5 size-4' />
          {t('usageMonitor.addDisplayField')}
        </Button>
      )}
    </div>
  );
});
