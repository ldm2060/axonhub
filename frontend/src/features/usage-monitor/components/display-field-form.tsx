'use client';

import { Plus, Trash2, ChevronDown } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useState } from 'react';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Collapsible, CollapsibleContent, CollapsibleTrigger } from '@/components/ui/collapsible';
import type { DisplayField, Variable } from '../data/schema';

interface Props {
  displayFields: DisplayField[];
  variables: Variable[];
  onChange: (displayFields: DisplayField[]) => void;
  readOnly?: boolean;
}

function generateKey(): string {
  return `df_${Date.now()}_${Math.random().toString(36).slice(2, 8)}`;
}

export function DisplayFieldForm({ displayFields, variables, onChange, readOnly }: Props) {
  const { t } = useTranslation();

  function updateField(index: number, updates: Partial<DisplayField>) {
    const next = [...displayFields];
    next[index] = { ...next[index], ...updates };
    onChange(next);
  }

  function removeField(index: number) {
    onChange(displayFields.filter((_, i) => i !== index));
  }

  function addField() {
    const newField: DisplayField = {
      key: generateKey(),
      label: '',
      valueRef: '',
      format: 'text',
      displayOrder: displayFields.length,
    };
    onChange([...displayFields, newField]);
  }

  return (
    <div className="space-y-4">
      {displayFields.map((df, index) => (
        <DisplayFieldItem
          key={df.key}
          df={df}
          index={index}
          variables={variables}
          readOnly={readOnly}
          onUpdate={(updates) => updateField(index, updates)}
          onRemove={() => removeField(index)}
        />
      ))}

      {!readOnly && (
        <Button type="button" variant="outline" onClick={addField} className="w-full">
          <Plus className="mr-1.5 size-4" />
          {t('usageMonitor.addDisplayField')}
        </Button>
      )}
    </div>
  );
}

interface DisplayFieldItemProps {
  df: DisplayField;
  index: number;
  variables: Variable[];
  readOnly?: boolean;
  onUpdate: (updates: Partial<DisplayField>) => void;
  onRemove: () => void;
}

function DisplayFieldItem({ df, index, variables, readOnly, onUpdate, onRemove }: DisplayFieldItemProps) {
  const { t } = useTranslation();
  const [badgeOpen, setBadgeOpen] = useState(false);

  // Determine if valueRef matches a variable key
  const isVariableRef = variables.some((v) => v.key === df.valueRef);

  return (
    <div className="rounded-lg border p-3 space-y-3">
      <div className="flex items-center justify-between">
        <span className="text-sm font-medium">
          {t('usageMonitor.displayField.label')} #{index + 1}
        </span>
        {!readOnly && (
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            onClick={onRemove}
            className="text-destructive hover:text-destructive"
            aria-label={t('usageMonitor.deleteDisplayField')}
          >
            <Trash2 className="size-4" />
          </Button>
        )}
      </div>

      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
        {/* Display Label */}
        <div className="space-y-1.5">
          <Label>{t('usageMonitor.field.label')}</Label>
          <Input
            value={df.label}
            onChange={(e) => onUpdate({ label: e.target.value })}
            placeholder={t('usageMonitor.field.label')}
            disabled={readOnly}
          />
        </div>

        {/* Display Format */}
        <div className="space-y-1.5">
          <Label>{t('usageMonitor.field.format')}</Label>
          <Select
            value={df.format}
            onValueChange={(v) => onUpdate({ format: v as DisplayField['format'] })}
            disabled={readOnly}
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

        {/* Value Ref - variable selector */}
        <div className="space-y-1.5">
          <Label>{t('usageMonitor.displayField.valueRef')}</Label>
          <Select
            value={isVariableRef ? df.valueRef : '__expression__'}
            onValueChange={(val) => {
              if (val === '__expression__') {
                // Switch to expression mode - clear valueRef so user types expression
                onUpdate({ valueRef: '' });
              } else {
                onUpdate({ valueRef: val });
              }
            }}
            disabled={readOnly}
          >
            <SelectTrigger className="w-full">
              <SelectValue placeholder={t('usageMonitor.displayField.valueRef')} />
            </SelectTrigger>
            <SelectContent>
              {variables.map((v) => (
                <SelectItem key={v.key} value={v.key}>
                  {v.key}
                </SelectItem>
              ))}
              <SelectItem value="__expression__">{t('usageMonitor.displayField.expression')}</SelectItem>
            </SelectContent>
          </Select>
        </div>

        {/* Expression Input - shown when valueRef is not a variable key */}
        {!isVariableRef && (
          <div className="space-y-1.5">
            <Label>{t('usageMonitor.field.expression')}</Label>
            <Input
              value={df.valueRef}
              onChange={(e) => onUpdate({ valueRef: e.target.value })}
              placeholder="${var1}/${var2}*100"
              className="font-mono"
              disabled={readOnly}
            />
            <p className="text-xs text-muted-foreground">{t('usageMonitor.field.expressionHint')}</p>
          </div>
        )}

        {/* Unit */}
        <div className="space-y-1.5">
          <Label>{t('usageMonitor.field.unit')}</Label>
          <Input
            value={df.unit ?? ''}
            onChange={(e) => onUpdate({ unit: e.target.value || undefined })}
            placeholder="%"
            disabled={readOnly}
          />
        </div>

        {/* Total Ref - only for fraction format */}
        {df.format === 'fraction' && (
          <div className="space-y-1.5">
            <Label>{t('usageMonitor.displayField.totalRef')}</Label>
            <Select
              value={df.totalRef ?? ''}
              onValueChange={(val) => onUpdate({ totalRef: val || undefined })}
              disabled={readOnly}
            >
              <SelectTrigger className="w-full">
                <SelectValue placeholder={t('usageMonitor.displayField.totalRef')} />
              </SelectTrigger>
              <SelectContent>
                {variables.map((v) => (
                  <SelectItem key={v.key} value={v.key}>
                    {v.key}
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
            type="button"
            variant="ghost"
            size="sm"
            className="w-full justify-start px-0 text-xs text-muted-foreground hover:text-foreground"
          >
            <ChevronDown className={`mr-1 size-3 transition-transform ${badgeOpen ? 'rotate-0' : '-rotate-90'}`} />
            {t('usageMonitor.displayField.badgeConfig')}
          </Button>
        </CollapsibleTrigger>
        <CollapsibleContent>
          <div className="grid grid-cols-1 gap-3 pt-2 sm:grid-cols-2">
            <div className="space-y-1.5">
              <Label>{t('usageMonitor.displayField.badgeKey')}</Label>
              <Input
                value={df.badge ?? ''}
                onChange={(e) => onUpdate({ badge: e.target.value || undefined })}
                placeholder="tier"
                className="font-mono"
                disabled={readOnly}
              />
            </div>
            <div className="space-y-1.5">
              <Label>{t('usageMonitor.displayField.badgePresets')}</Label>
              <Input
                value={df.badgePresets ?? ''}
                onChange={(e) => onUpdate({ badgePresets: e.target.value || undefined })}
                placeholder='{"free":"gradient","pro":"blue"}'
                className="font-mono text-xs"
                disabled={readOnly}
              />
            </div>
          </div>
        </CollapsibleContent>
      </Collapsible>
    </div>
  );
}
