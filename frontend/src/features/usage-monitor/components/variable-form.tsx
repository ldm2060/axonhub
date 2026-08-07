'use client';

import { Plus, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import type { Variable } from '../data/schema';

interface Props {
  variables: Variable[];
  onChange: (variables: Variable[]) => void;
  readOnly?: boolean;
}

export function VariableForm({ variables, onChange, readOnly }: Props) {
  const { t } = useTranslation();

  function updateVariable(index: number, updates: Partial<Variable>) {
    const next = [...variables];
    next[index] = { ...next[index], ...updates };
    onChange(next);
  }

  function removeVariable(index: number) {
    onChange(variables.filter((_, i) => i !== index));
  }

  function addVariable() {
    onChange([...variables, { key: '', path: '', type: 'jsonpath' }]);
  }

  return (
    <div className='space-y-4'>
      {variables.map((variable, index) => (
        <div key={index} className='space-y-3 rounded-lg border p-3'>
          <div className='flex items-center justify-between'>
            <span className='text-sm font-medium'>
              {t('usageMonitor.variable.label')} #{index + 1}
            </span>
            {!readOnly && (
              <Button
                type='button'
                variant='ghost'
                size='icon-sm'
                onClick={() => removeVariable(index)}
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
                onChange={(e) => updateVariable(index, { key: e.target.value })}
                placeholder='var_name'
                disabled={readOnly}
              />
            </div>

            {/* Parse Type */}
            <div className='space-y-1.5'>
              <Label>{t('usageMonitor.field.type')}</Label>
              <Select
                value={variable.type}
                onValueChange={(v) => updateVariable(index, { type: v as Variable['type'] })}
                disabled={readOnly}
              >
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
                onChange={(e) => updateVariable(index, { path: e.target.value })}
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
                    updateVariable(index, { groupIndex: parsed.length > 0 ? parsed : undefined });
                  }}
                  placeholder='1,2'
                  className='font-mono'
                  disabled={readOnly}
                />
              </div>
            )}
          </div>
        </div>
      ))}

      {!readOnly && (
        <Button type='button' variant='outline' onClick={addVariable} className='w-full'>
          <Plus className='mr-1.5 size-4' />
          {t('usageMonitor.addVariable')}
        </Button>
      )}
    </div>
  );
}
