'use client';

import { memo, useCallback, type Dispatch, type SetStateAction } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import type { Variable } from '../data/schema';

interface Props {
  variables: Variable[];
  onChange: Dispatch<SetStateAction<Variable[]>>;
  readOnly?: boolean;
}

interface VariableItemProps {
  variable: Variable;
  index: number;
  readOnly?: boolean;
  onUpdate: (index: number, updates: Partial<Variable>) => void;
  onRemove: (index: number) => void;
}

// Memoized so editing one row (or unrelated dialog state) doesn't re-render every row.
const VariableItem = memo(function VariableItem({ variable, index, readOnly, onUpdate, onRemove }: VariableItemProps) {
  const { t } = useTranslation();

  return (
    <div className='space-y-3 rounded-lg border p-3'>
      <div className='flex items-center justify-between'>
        <span className='text-sm font-medium'>
          {t('usageMonitor.variable.label')} #{index + 1}
        </span>
        {!readOnly && (
          <Button
            type='button'
            variant='ghost'
            size='icon-sm'
            onClick={() => onRemove(index)}
            className='text-destructive hover:text-destructive'
            aria-label={t('usageMonitor.deleteVariable')}
          >
            <Trash2 className='size-4' />
          </Button>
        )}
      </div>

      <div className='grid grid-cols-1 gap-3 sm:grid-cols-2'>
        {/* Key */}
        <div className='space-y-1.5'>
          <Label>{t('usageMonitor.variable.key')}</Label>
          <Input
            value={variable.key}
            onChange={(e) => onUpdate(index, { key: e.target.value })}
            placeholder='var_name'
            disabled={readOnly}
          />
        </div>

        {/* Parse Type */}
        <div className='space-y-1.5'>
          <Label>{t('usageMonitor.field.type')}</Label>
          <Select value={variable.type} onValueChange={(v) => onUpdate(index, { type: v as Variable['type'] })} disabled={readOnly}>
            <SelectTrigger className='w-full'>
              <SelectValue />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value='jsonpath'>{t('usageMonitor.field.jsonpath')}</SelectItem>
              <SelectItem value='regex'>{t('usageMonitor.field.regex')}</SelectItem>
            </SelectContent>
          </Select>
        </div>

        {/* Parse Path */}
        <div className='space-y-1.5 sm:col-span-2'>
          <Label>{t('usageMonitor.field.path')}</Label>
          <Input
            value={variable.path}
            onChange={(e) => onUpdate(index, { path: e.target.value })}
            placeholder='$.data.value'
            className='font-mono'
            disabled={readOnly}
          />
        </div>

        {/* Group Index - only for regex type */}
        {variable.type === 'regex' && (
          <div className='space-y-1.5'>
            <Label>{t('usageMonitor.groupIndex')}</Label>
            <Input
              value={variable.groupIndex?.join(',') ?? ''}
              onChange={(e) => {
                const parsed = e.target.value
                  .split(',')
                  .map((s) => parseInt(s.trim(), 10))
                  .filter((n) => !isNaN(n));
                onUpdate(index, { groupIndex: parsed.length > 0 ? parsed : undefined });
              }}
              placeholder='1,2'
              className='font-mono'
              disabled={readOnly}
            />
          </div>
        )}
      </div>
    </div>
  );
});

// Memoized so typing in unrelated dialog fields (name, API key, ...) skips
// re-rendering the whole variable list, which is large for template channels.
export const VariableForm = memo(function VariableForm({ variables, onChange, readOnly }: Props) {
  const { t } = useTranslation();

  const updateVariable = useCallback(
    (index: number, updates: Partial<Variable>) => {
      onChange((prev) => {
        const next = [...prev];
        next[index] = { ...next[index], ...updates };
        return next;
      });
    },
    [onChange]
  );

  const removeVariable = useCallback((index: number) => onChange((prev) => prev.filter((_, i) => i !== index)), [onChange]);

  const addVariable = useCallback(() => onChange((prev) => [...prev, { key: '', path: '', type: 'jsonpath' }]), [onChange]);

  return (
    <div className='space-y-4'>
      {variables.map((variable, index) => (
        <VariableItem
          key={index}
          variable={variable}
          index={index}
          readOnly={readOnly}
          onUpdate={updateVariable}
          onRemove={removeVariable}
        />
      ))}

      {!readOnly && (
        <Button type='button' variant='outline' onClick={addVariable} className='w-full'>
          <Plus className='mr-1.5 size-4' />
          {t('usageMonitor.addVariable')}
        </Button>
      )}
    </div>
  );
});
