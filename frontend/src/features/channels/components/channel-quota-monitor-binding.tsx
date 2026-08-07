'use client';

import { useMemo, useRef } from 'react';
import { Plus, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Tooltip, TooltipContent, TooltipTrigger } from '@/components/ui/tooltip';
import { useUsageMonitorChannels } from '@/features/usage-monitor/data/usage-monitor';
import type { SaveChannelQuotaMonitorBindingInput } from '../data/schema';
import type { QuotaMonitorConditionOperator, QuotaMonitorBindingTriggerStatus } from '../data/schema';

const TRIGGER_STATUS_OPTIONS: QuotaMonitorBindingTriggerStatus[] = ['available', 'warning', 'exhausted', 'unknown'];
const OPERATOR_OPTIONS: QuotaMonitorConditionOperator[] = ['<', '<=', '=', '!=', '>=', '>', 'contains', 'not_contains'];
const BUILTIN_FIELDS = ['maxUsageRatio'];

/** Generate a stable local key for each binding row. */
let _nextBindingKey = 0;
function newBindingKey(): string {
  return `bkey_${++_nextBindingKey}_${Date.now().toString(36)}`;
}

interface BindingWithKey extends SaveChannelQuotaMonitorBindingInput {
  _key: string;
}

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

  // Maintain stable keys for each binding row so React doesn't lose DOM state
  // when array indices shift. KeyedBindings is derived from the parent's
  // bindings array plus a local _key per entry.
  const keyMapRef = useRef<Map<number, string>>(new Map());

  // Assign stable keys: existing indices keep their key, new entries get a fresh one.
  const keyedBindings: BindingWithKey[] = useMemo(() => {
    // Clean up keys for removed indices
    for (const k of keyMapRef.current.keys()) {
      if (k >= bindings.length) keyMapRef.current.delete(k);
    }

    return bindings.map((b, i) => {
      let key = keyMapRef.current.get(i);
      if (!key) {
        // If index is within previous length, it's likely a shift — try to keep
        // stable by only assigning a new key for truly new entries.
        key = newBindingKey();
        keyMapRef.current.set(i, key);
      }
      return { ...b, _key: key };
    });
  }, [bindings]);

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
    // Remove the key for the deleted index and re-key remaining entries
    const newKeyMap = new Map<number, string>();
    let srcIdx = 0;
    for (let dstIdx = 0; dstIdx < next.length; dstIdx++) {
      // Skip the removed index
      if (srcIdx === index) srcIdx++;
      const existingKey = keyMapRef.current.get(srcIdx);
      if (existingKey) newKeyMap.set(dstIdx, existingKey);
      srcIdx++;
    }
    keyMapRef.current = newKeyMap;
    onBindingsChange(next);
  };

  const handleBindingFieldChange = <K extends keyof SaveChannelQuotaMonitorBindingInput>(
    index: number,
    field: K,
    value: SaveChannelQuotaMonitorBindingInput[K]
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

  const handleConditionChange = (bindingIndex: number, conditionIndex: number, field: 'field' | 'operator' | 'value', value: string) => {
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

  const removeLabel = t('channels.quotaMonitorBinding.remove');

  return (
    <div className='space-y-2'>
      <div className='flex items-center gap-2'>
        <Switch checked={enabled} onCheckedChange={onEnabledChange} />
        <Label className='text-sm'>{t('channels.quotaMonitorBinding.enabled')}</Label>
      </div>
      <p className='text-muted-foreground text-xs'>{t('channels.quotaMonitorBinding.description')}</p>

      {enabled && (
        <div className='space-y-3 rounded-md border p-3'>
          <div className='space-y-1.5'>
            <Label className='text-sm'>{t('channels.quotaMonitorBinding.strategy')}</Label>
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
            {bindings.length === 0 && <p className='text-muted-foreground text-sm'>{t('channels.quotaMonitorBinding.empty')}</p>}

            {keyedBindings.map((binding, bindingIndex) => {
              const _selectedMonitor = monitorMap.get(binding.usageMonitorChannelID);
              const fieldSuggestions = getFieldSuggestions(binding.usageMonitorChannelID);

              return (
                <div key={binding._key} className='border-border space-y-3 rounded-md border p-3'>
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
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <Button
                          type='button'
                          variant='ghost'
                          size='icon'
                          className='mt-4 h-8 w-8 shrink-0'
                          onClick={() => handleRemoveBinding(bindingIndex)}
                        >
                          <Trash2 className='h-4 w-4' />
                        </Button>
                      </TooltipTrigger>
                      <TooltipContent>{removeLabel}</TooltipContent>
                    </Tooltip>
                  </div>

                  {/* Binding-level enabled toggle */}
                  <div className='flex items-center gap-2'>
                    <Switch checked={binding.enabled} onCheckedChange={(v) => handleBindingFieldChange(bindingIndex, 'enabled', v)} />
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
                            className='cursor-pointer text-xs select-none'
                            onClick={() => handleToggleTriggerStatus(bindingIndex, status)}
                          >
                            {t(`channels.quotaMonitorBinding.statuses.${status}`)}
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
                          list={`field-suggestions-${binding._key}-${conditionIndex}`}
                          value={condition.field}
                          onChange={(e) => handleConditionChange(bindingIndex, conditionIndex, 'field', e.target.value)}
                          placeholder={t('channels.quotaMonitorBinding.field')}
                          className='h-7 flex-1 text-xs'
                        />
                        <datalist id={`field-suggestions-${binding._key}-${conditionIndex}`}>
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
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <Button
                              type='button'
                              variant='ghost'
                              size='icon'
                              className='h-7 w-7 shrink-0'
                              onClick={() => handleRemoveCondition(bindingIndex, conditionIndex)}
                            >
                              <Trash2 className='h-3 w-3' />
                            </Button>
                          </TooltipTrigger>
                          <TooltipContent>{removeLabel}</TooltipContent>
                        </Tooltip>
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
        </div>
      )}
    </div>
  );
}
