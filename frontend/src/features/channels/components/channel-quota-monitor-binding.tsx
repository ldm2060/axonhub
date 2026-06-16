'use client';

import { useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Plus, Trash2 } from 'lucide-react';
import { useUsageMonitorChannels } from '@/features/usage-monitor/data/usage-monitor';
import type { SaveChannelQuotaMonitorBindingInput } from '../data/schema';
import type { QuotaMonitorConditionOperator, QuotaMonitorBindingTriggerStatus } from '../data/schema';

const TRIGGER_STATUS_OPTIONS: QuotaMonitorBindingTriggerStatus[] = ['available', 'warning', 'exhausted', 'unknown'];
const OPERATOR_OPTIONS: QuotaMonitorConditionOperator[] = ['<', '<=', '=', '!=', '>=', '>', 'contains', 'not_contains'];
const BUILTIN_FIELDS = ['maxUsageRatio'];

interface ChannelQuotaMonitorBindingProps {
  enabled: boolean;
  strategy: 'any' | 'all';
  bindings: SaveChannelQuotaMonitorBindingInput[];
  onEnabledChange: (enabled: boolean) => void;
  onStrategyChange: (strategy: 'any' | 'all') => void;
  onBindingsChange: (bindings: SaveChannelQuotaMonitorBindingInput[]) => void;
}

export function ChannelQuotaMonitorBinding({
  enabled,
  strategy,
  bindings,
  onEnabledChange,
  onStrategyChange,
  onBindingsChange,
}: ChannelQuotaMonitorBindingProps) {
  const { t } = useTranslation();
  const { data: monitors } = useUsageMonitorChannels();

  // Build a map of monitor ID -> monitor for quick lookup and field suggestions
  const monitorMap = useMemo(() => {
    const map = new Map<string, { name: string; displayFieldKeys: string[] }>();
    for (const m of monitors ?? []) {
      map.set(m.id, {
        name: m.name,
        displayFieldKeys: (m.displayFields ?? []).map((df) => df.key),
      });
    }
    return map;
  }, [monitors]);

  const handleAddBinding = () => {
    const next = [
      ...bindings,
      {
        usageMonitorChannelID: '',
        enabled: true,
        triggerStatuses: ['warning', 'exhausted'] as QuotaMonitorBindingTriggerStatus[],
        conditions: [],
      },
    ];
    onBindingsChange(next);
  };

  const handleRemoveBinding = (index: number) => {
    const next = bindings.filter((_, i) => i !== index);
    onBindingsChange(next);
  };

  const handleBindingFieldChange = <K extends keyof SaveChannelQuotaMonitorBindingInput>(
    index: number,
    field: K,
    value: SaveChannelQuotaMonitorBindingInput[K],
  ) => {
    const next = bindings.map((b, i) => (i === index ? { ...b, [field]: value } : b));
    onBindingsChange(next);
  };

  const handleToggleTriggerStatus = (bindingIndex: number, status: QuotaMonitorBindingTriggerStatus) => {
    const binding = bindings[bindingIndex];
    const current = binding.triggerStatuses ?? [];
    const nextStatuses = current.includes(status) ? current.filter((s) => s !== status) : [...current, status];
    handleBindingFieldChange(bindingIndex, 'triggerStatuses', nextStatuses);
  };

  const handleAddCondition = (bindingIndex: number) => {
    const binding = bindings[bindingIndex];
    const nextConditions = [...(binding.conditions ?? []), { field: '', operator: '=' as QuotaMonitorConditionOperator, value: '' }];
    handleBindingFieldChange(bindingIndex, 'conditions', nextConditions);
  };

  const handleRemoveCondition = (bindingIndex: number, conditionIndex: number) => {
    const binding = bindings[bindingIndex];
    const nextConditions = (binding.conditions ?? []).filter((_, i) => i !== conditionIndex);
    handleBindingFieldChange(bindingIndex, 'conditions', nextConditions);
  };

  const handleConditionChange = (
    bindingIndex: number,
    conditionIndex: number,
    field: 'field' | 'operator' | 'value',
    value: string,
  ) => {
    const binding = bindings[bindingIndex];
    const conditions = [...(binding.conditions ?? [])];
    conditions[conditionIndex] = { ...conditions[conditionIndex], [field]: value };
    handleBindingFieldChange(bindingIndex, 'conditions', conditions);
  };

  const getFieldSuggestions = (monitorID: string): string[] => {
    const monitor = monitorMap.get(monitorID);
    if (!monitor) return BUILTIN_FIELDS;
    return [...BUILTIN_FIELDS, ...monitor.displayFieldKeys];
  };

  return (
    <div className='space-y-4'>
      <div>
        <h3 className='text-lg font-medium'>{t('channels.quotaMonitorBinding.title')}</h3>
        <p className='text-muted-foreground text-sm'>{t('channels.quotaMonitorBinding.description')}</p>
      </div>

      <div className='flex items-center space-x-2'>
        <Switch checked={enabled} onCheckedChange={onEnabledChange} />
        <Label>{t('channels.quotaMonitorBinding.enabled')}</Label>
      </div>

      {enabled && (
        <>
          <div className='space-y-2'>
            <Label>{t('channels.quotaMonitorBinding.strategy')}</Label>
            <Select value={strategy} onValueChange={(v) => onStrategyChange(v as 'any' | 'all')}>
              <SelectTrigger>
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value='any'>{t('channels.quotaMonitorBinding.strategyAny')}</SelectItem>
                <SelectItem value='all'>{t('channels.quotaMonitorBinding.strategyAll')}</SelectItem>
              </SelectContent>
            </Select>
          </div>

          <div className='space-y-3'>
            {bindings.length === 0 && (
              <p className='text-muted-foreground text-sm'>{t('channels.quotaMonitorBinding.empty')}</p>
            )}

            {bindings.map((binding, bindingIndex) => {
              const selectedMonitor = monitorMap.get(binding.usageMonitorChannelID);
              const fieldSuggestions = getFieldSuggestions(binding.usageMonitorChannelID);

              return (
                <div key={bindingIndex} className='border-border rounded-md border p-3 space-y-3'>
                  {/* Monitor select + remove */}
                  <div className='flex items-center gap-2'>
                    <div className='flex-1'>
                      <Label className='text-xs'>{t('channels.quotaMonitorBinding.monitor')}</Label>
                      <Select
                        value={binding.usageMonitorChannelID}
                        onValueChange={(v) => handleBindingFieldChange(bindingIndex, 'usageMonitorChannelID', v)}
                      >
                        <SelectTrigger className='h-8'>
                          <SelectValue placeholder={t('channels.quotaMonitorBinding.monitor')} />
                        </SelectTrigger>
                        <SelectContent>
                          {(monitors ?? []).map((m) => (
                            <SelectItem key={m.id} value={m.id}>
                              {m.name}
                            </SelectItem>
                          ))}
                        </SelectContent>
                      </Select>
                    </div>
                    <Button
                      type='button'
                      variant='ghost'
                      size='icon'
                      className='mt-4 h-8 w-8 shrink-0'
                      onClick={() => handleRemoveBinding(bindingIndex)}
                    >
                      <Trash2 className='h-4 w-4' />
                    </Button>
                  </div>

                  {/* Binding-level enabled toggle */}
                  <div className='flex items-center space-x-2'>
                    <Switch
                      checked={binding.enabled}
                      onCheckedChange={(v) => handleBindingFieldChange(bindingIndex, 'enabled', v)}
                    />
                    <Label className='text-xs'>{t('channels.quotaMonitorBinding.enabled')}</Label>
                  </div>

                  {/* Trigger Statuses */}
                  <div className='space-y-1'>
                    <Label className='text-xs'>{t('channels.quotaMonitorBinding.triggerStatuses')}</Label>
                    <div className='flex flex-wrap gap-1.5'>
                      {TRIGGER_STATUS_OPTIONS.map((status) => {
                        const isActive = (binding.triggerStatuses ?? []).includes(status);
                        return (
                          <Badge
                            key={status}
                            variant={isActive ? 'default' : 'outline'}
                            className='cursor-pointer select-none text-xs'
                            onClick={() => handleToggleTriggerStatus(bindingIndex, status)}
                          >
                            {status}
                          </Badge>
                        );
                      })}
                    </div>
                  </div>

                  {/* Field Conditions */}
                  <div className='space-y-2'>
                    <Label className='text-xs'>{t('channels.quotaMonitorBinding.fieldConditions')}</Label>
                    {(binding.conditions ?? []).map((condition, conditionIndex) => (
                      <div key={conditionIndex} className='flex items-center gap-1.5'>
                        <Input
                          list={`field-suggestions-${bindingIndex}-${conditionIndex}`}
                          value={condition.field}
                          onChange={(e) => handleConditionChange(bindingIndex, conditionIndex, 'field', e.target.value)}
                          placeholder={t('channels.quotaMonitorBinding.field')}
                          className='h-7 flex-1 text-xs'
                        />
                        <datalist id={`field-suggestions-${bindingIndex}-${conditionIndex}`}>
                          {fieldSuggestions.map((f) => (
                            <option key={f} value={f} />
                          ))}
                        </datalist>
                        <Select
                          value={condition.operator}
                          onValueChange={(v) => handleConditionChange(bindingIndex, conditionIndex, 'operator', v)}
                        >
                          <SelectTrigger className='h-7 w-20 text-xs'>
                            <SelectValue />
                          </SelectTrigger>
                          <SelectContent>
                            {OPERATOR_OPTIONS.map((op) => (
                              <SelectItem key={op} value={op}>
                                {op}
                              </SelectItem>
                            ))}
                          </SelectContent>
                        </Select>
                        <Input
                          value={condition.value}
                          onChange={(e) => handleConditionChange(bindingIndex, conditionIndex, 'value', e.target.value)}
                          placeholder={t('channels.quotaMonitorBinding.value')}
                          className='h-7 flex-1 text-xs'
                        />
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon'
                          className='h-7 w-7 shrink-0'
                          onClick={() => handleRemoveCondition(bindingIndex, conditionIndex)}
                        >
                          <Trash2 className='h-3 w-3' />
                        </Button>
                      </div>
                    ))}
                    <Button
                      type='button'
                      variant='outline'
                      size='sm'
                      className='h-7 text-xs'
                      onClick={() => handleAddCondition(bindingIndex)}
                    >
                      <Plus className='mr-1 h-3 w-3' />
                      {t('channels.quotaMonitorBinding.addCondition')}
                    </Button>
                  </div>
                </div>
              );
            })}

            <Button type='button' variant='outline' size='sm' onClick={handleAddBinding}>
              <Plus className='mr-2 h-4 w-4' />
              {t('channels.quotaMonitorBinding.addBinding')}
            </Button>
          </div>
        </>
      )}
    </div>
  );
}
