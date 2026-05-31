'use client';

import { Trash2, Plus } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import type { FieldConfig } from '../data/schema';

interface Props {
  fields: FieldConfig[];
  onChange: (fields: FieldConfig[]) => void;
}

function generateKey(): string {
  return `field_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

export function FieldConfigForm({ fields, onChange }: Props) {
  const { t } = useTranslation();

  function updateField(index: number, updates: Partial<FieldConfig>) {
    const next = [...fields];
    next[index] = { ...next[index], ...updates };
    onChange(next);
  }

  function removeField(index: number) {
    onChange(fields.filter((_, i) => i !== index));
  }

  function addField() {
    const newField: FieldConfig = {
      key: generateKey(),
      label: '',
      path: '',
      type: 'jsonpath',
      format: 'text',
      displayOrder: fields.length,
    };
    onChange([...fields, newField]);
  }

  return (
    <div className="space-y-4">
      {fields.map((field, index) => (
        <div key={field.key} className="rounded-lg border p-3 space-y-3">
          <div className="flex items-center justify-between">
            <span className="text-sm font-medium">
              {t('usageMonitor.field.label')} #{index + 1}
            </span>
            <Button
              type="button"
              variant="ghost"
              size="icon-sm"
              onClick={() => removeField(index)}
              className="text-destructive hover:text-destructive"
              aria-label={t('usageMonitor.deleteField')}
            >
              <Trash2 className="size-4" />
            </Button>
          </div>

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            {/* Display Name */}
            <div className="space-y-1.5">
              <Label>{t('usageMonitor.field.label')}</Label>
              <Input
                value={field.label}
                onChange={(e) => updateField(index, { label: e.target.value })}
                placeholder={t('usageMonitor.field.label')}
              />
            </div>

            {/* Parse Type */}
            <div className="space-y-1.5">
              <Label>{t('usageMonitor.field.type')}</Label>
              <Select
                value={field.type}
                onValueChange={(v) => updateField(index, { type: v as FieldConfig['type'] })}
              >
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="jsonpath">{t('usageMonitor.field.jsonpath')}</SelectItem>
                  <SelectItem value="regex">{t('usageMonitor.field.regex')}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {/* Parse Path */}
            <div className="space-y-1.5">
              <Label>{t('usageMonitor.field.path')}</Label>
              <Input
                value={field.path}
                onChange={(e) => updateField(index, { path: e.target.value })}
                placeholder="$.data.value"
                className="font-mono"
              />
            </div>

            {/* Display Format */}
            <div className="space-y-1.5">
              <Label>{t('usageMonitor.field.format')}</Label>
              <Select
                value={field.format}
                onValueChange={(v) => updateField(index, { format: v as FieldConfig['format'] })}
              >
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="percentage">{t('usageMonitor.format.percentage')}</SelectItem>
                  <SelectItem value="fraction">{t('usageMonitor.format.fraction')}</SelectItem>
                  <SelectItem value="number">{t('usageMonitor.format.number')}</SelectItem>
                  <SelectItem value="datetime">{t('usageMonitor.format.datetime')}</SelectItem>
                  <SelectItem value="text">{t('usageMonitor.format.text')}</SelectItem>
                </SelectContent>
              </Select>
            </div>

            {/* Total Path - only for percentage format */}
            {field.format === 'percentage' && (
              <div className="space-y-1.5">
                <Label>{t('usageMonitor.totalPath')}</Label>
                <Input
                  value={field.totalPath ?? ''}
                  onChange={(e) => updateField(index, { totalPath: e.target.value })}
                  placeholder="$.data.total"
                  className="font-mono"
                />
              </div>
            )}

            {/* Group Index - only for regex type */}
            {field.type === 'regex' && (
              <div className="space-y-1.5">
                <Label>{t('usageMonitor.groupIndex')}</Label>
                <Input
                  value={field.groupIndex?.join(',') ?? ''}
                  onChange={(e) => {
                    const parsed = e.target.value
                      .split(',')
                      .map((s) => parseInt(s.trim(), 10))
                      .filter((n) => !isNaN(n));
                    updateField(index, { groupIndex: parsed.length > 0 ? parsed : undefined });
                  }}
                  placeholder="1,2"
                  className="font-mono"
                />
              </div>
            )}

            {/* Unit */}
            <div className="space-y-1.5">
              <Label>{t('usageMonitor.field.unit')}</Label>
              <Input
                value={field.unit ?? ''}
                onChange={(e) => updateField(index, { unit: e.target.value || undefined })}
                placeholder="%"
              />
            </div>
          </div>
        </div>
      ))}

      <Button type="button" variant="outline" onClick={addField} className="w-full">
        <Plus className="mr-1.5 size-4" />
        {t('usageMonitor.addField')}
      </Button>
    </div>
  );
}
