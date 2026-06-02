'use client';

import { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Eye, EyeOff, Lock } from 'lucide-react';
import { useUpdateUsageMonitorChannel } from '../data/usage-monitor';
import { useUsageMonitorContext } from '../context/usage-monitor-context';
import type { Variable, DisplayField, VariableInput, DisplayFieldInput, FieldConfig } from '../data/schema';
import { VariableForm } from './variable-form';
import { DisplayFieldForm } from './display-field-form';
import { TestConnection } from './test-connection';

const MASKED_API_KEY = '••••••••';

export function EditChannelDialog() {
  const { t } = useTranslation();
  const { open, setOpen, currentChannel } = useUsageMonitorContext();
  const updateMutation = useUpdateUsageMonitorChannel();

  const isOpen = open === 'edit';
  const source = currentChannel?.source;

  const isTemplate = source === 'template';
  const isBuiltin = source === 'builtin';

  const [name, setName] = useState('');
  const [apiUrl, setApiUrl] = useState('');
  const [apiMethod, setApiMethod] = useState<'GET' | 'POST'>('GET');
  const [apiHeaders, setApiHeaders] = useState('');
  const [apiBody, setApiBody] = useState('');
  const [pollIntervalMin, setPollIntervalMin] = useState(5);
  const [variables, setVariables] = useState<Variable[]>([]);
  const [displayFields, setDisplayFields] = useState<DisplayField[]>([]);
  const [headersError, setHeadersError] = useState('');

  // API key state for template channels
  const [apiKey, setApiKey] = useState('');
  const [showApiKey, setShowApiKey] = useState(false);

  // Populate form when channel changes
  useEffect(() => {
    if (!isOpen || !currentChannel) return;
    setName(currentChannel.name || '');
    setApiUrl(currentChannel.apiUrl || '');
    setApiMethod(currentChannel.apiMethod || 'GET');
    setApiHeaders(currentChannel.apiHeaders || '');
    setApiBody(currentChannel.apiBody || '');
    setPollIntervalMin(Math.round((currentChannel.pollInterval || 300) / 60));
    setVariables(currentChannel.variables ?? []);
    setDisplayFields(currentChannel.displayFields ?? []);
    setHeadersError('');
    // For template channels, pre-fill with masked API key
    setApiKey(currentChannel.apiKey ?? MASKED_API_KEY);
    setShowApiKey(false);
  }, [isOpen, currentChannel]);

  function validateHeaders(value: string) {
    setApiHeaders(value);
    if (!value.trim()) {
      setHeadersError('');
      return;
    }
    try {
      JSON.parse(value);
      setHeadersError('');
    } catch {
      setHeadersError('Invalid JSON');
    }
  }

  async function handleSubmit() {
    if (!currentChannel) return;
    try {
      if (isTemplate) {
        // Template channels: only send name, apiKey (if changed), pollInterval, and displayFields
        // Do NOT send variables — backend ignores them anyway
        const displayFieldInputs: DisplayFieldInput[] = displayFields.map((df) => ({
          key: df.key,
          label: df.label,
          valueRef: df.valueRef,
          format: df.format,
          unit: df.unit,
          totalRef: df.totalRef,
          displayOrder: df.displayOrder,
          badge: df.badge,
          badgePresets: df.badgePresets,
        }));
        await updateMutation.mutateAsync({
          id: currentChannel.id,
          input: {
            name,
            pollInterval: pollIntervalMin * 60,
            displayFields: displayFieldInputs,
            ...(apiKey.trim() && apiKey !== MASKED_API_KEY ? { apiKey } : {}),
          },
        });
      } else if (isBuiltin) {
        // Builtin channels: name, pollInterval, variables, displayFields
        const variableInputs: VariableInput[] = variables.map((v) => ({
          key: v.key,
          path: v.path,
          type: v.type,
          groupIndex: v.groupIndex,
        }));
        const displayFieldInputs: DisplayFieldInput[] = displayFields.map((df) => ({
          key: df.key,
          label: df.label,
          valueRef: df.valueRef,
          format: df.format,
          unit: df.unit,
          totalRef: df.totalRef,
          displayOrder: df.displayOrder,
          badge: df.badge,
          badgePresets: df.badgePresets,
        }));
        await updateMutation.mutateAsync({
          id: currentChannel.id,
          input: {
            name,
            pollInterval: pollIntervalMin * 60,
            variables: variableInputs,
            displayFields: displayFieldInputs,
          },
        });
      } else {
        // Custom channels: send everything including variables and displayFields
        const variableInputs: VariableInput[] = variables.map((v) => ({
          key: v.key,
          path: v.path,
          type: v.type,
          groupIndex: v.groupIndex,
        }));
        const displayFieldInputs: DisplayFieldInput[] = displayFields.map((df) => ({
          key: df.key,
          label: df.label,
          valueRef: df.valueRef,
          format: df.format,
          unit: df.unit,
          totalRef: df.totalRef,
          displayOrder: df.displayOrder,
          badge: df.badge,
          badgePresets: df.badgePresets,
        }));
        await updateMutation.mutateAsync({
          id: currentChannel.id,
          input: {
            name,
            apiUrl,
            apiMethod,
            apiHeaders,
            apiBody: apiBody || undefined,
            pollInterval: pollIntervalMin * 60,
            variables: variableInputs,
            displayFields: displayFieldInputs,
          },
        });
      }
      setOpen(null);
    } catch {
      // error handled by mutation
    }
  }

  const canSubmit = isTemplate
    ? name.trim()
    : isBuiltin
      ? name.trim()
      : name.trim() && apiUrl.trim() && !headersError;

  // Build fields for TestConnection (legacy format from variables)
  const testFields: FieldConfig[] = variables.map((v, i) => ({
    key: v.key,
    label: v.key,
    path: v.path,
    type: v.type,
    format: 'text',
    groupIndex: v.groupIndex,
    displayOrder: i,
  }));

  return (
    <Dialog
      open={isOpen}
      onOpenChange={(v) => {
        if (!v) setOpen(null);
      }}
    >
      <DialogContent className="flex max-h-[90vh] flex-col sm:max-w-2xl">
        <DialogHeader className="flex-shrink-0">
          <DialogTitle>{t('usageMonitor.editChannel')}</DialogTitle>
          <DialogDescription />
        </DialogHeader>

        <div className="min-h-0 flex-1 overflow-y-auto pr-1">
          <div className="space-y-5 pb-4">
            {/* Template channel: API Key */}
            {isTemplate && (
              <div className="space-y-1.5">
                <Label>{t('usageMonitor.apiKey')}</Label>
                <div className="relative">
                  <Input
                    type={showApiKey ? 'text' : 'password'}
                    value={apiKey}
                    onChange={(e) => setApiKey(e.target.value)}
                    placeholder={t('usageMonitor.apiKeyPlaceholder')}
                    className="pr-10 font-mono"
                  />
                  <button
                    type="button"
                    onClick={() => setShowApiKey(!showApiKey)}
                    className="absolute right-3 top-1/2 -translate-y-1/2 text-muted-foreground hover:text-foreground"
                  >
                    {showApiKey ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                  </button>
                </div>
                <p className="text-xs text-muted-foreground">
                  {apiKey === MASKED_API_KEY ? t('usageMonitor.apiKeyUnchangedHint') ?? 'Leave unchanged to keep current key' : ''}
                </p>
              </div>
            )}

            {/* Custom: API URL */}
            {!isTemplate && !isBuiltin && (
              <div className="space-y-1.5">
                <Label>{t('usageMonitor.apiUrl')}</Label>
                <Input
                  value={apiUrl}
                  onChange={(e) => setApiUrl(e.target.value)}
                  placeholder="https://api.example.com/v1/usage"
                  className="font-mono"
                />
              </div>
            )}

            {/* Custom: API Method */}
            {!isTemplate && !isBuiltin && (
              <div className="space-y-1.5">
                <Label>{t('usageMonitor.apiMethod')}</Label>
                <Select value={apiMethod} onValueChange={(v) => setApiMethod(v as 'GET' | 'POST')}>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="GET">GET</SelectItem>
                    <SelectItem value="POST">POST</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            )}

            {/* Custom: API Headers */}
            {!isTemplate && !isBuiltin && (
              <div className="space-y-1.5">
                <Label>{t('usageMonitor.apiHeaders')}</Label>
                <Textarea
                  value={apiHeaders}
                  onChange={(e) => validateHeaders(e.target.value)}
                  placeholder='{"Authorization": "Bearer sk-..."}'
                  className="font-mono min-h-20"
                />
                {headersError && (
                  <p className="text-xs text-destructive">{headersError}</p>
                )}
              </div>
            )}

            {/* Custom: API Body */}
            {!isTemplate && !isBuiltin && apiMethod === 'POST' && (
              <div className="space-y-1.5">
                <Label>{t('usageMonitor.apiBody')}</Label>
                <Textarea
                  value={apiBody}
                  onChange={(e) => setApiBody(e.target.value)}
                  placeholder='{"key": "value"}'
                  className="font-mono min-h-20"
                />
              </div>
            )}

            {/* Channel Name */}
            <div className="space-y-1.5">
              <Label>{t('usageMonitor.channelName')}</Label>
              <Input
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t('usageMonitor.channelName')}
              />
            </div>

            {/* Poll Interval */}
            <div className="space-y-1.5">
              <Label>{t('usageMonitor.pollInterval')}</Label>
              <Input
                type="number"
                min={1}
                value={pollIntervalMin}
                onChange={(e) => setPollIntervalMin(parseInt(e.target.value, 10) || 1)}
              />
              <p className="text-xs text-muted-foreground">{t('usageMonitor.pollIntervalUnit')}</p>
            </div>

            {/* Variable Section */}
            <div className="space-y-3">
              <div className="flex items-center gap-2">
                <Label className="text-sm font-semibold">{t('usageMonitor.variableSection')}</Label>
                {isTemplate && <Lock className="size-3.5 text-muted-foreground" />}
              </div>
              {isTemplate && (
                <p className="text-xs text-muted-foreground">{t('usageMonitor.templateVariablesHint')}</p>
              )}
              <VariableForm
                variables={variables}
                onChange={setVariables}
                readOnly={isTemplate}
              />
            </div>

            {/* Display Field Section */}
            <div className="space-y-3">
              <div className="flex items-center gap-2">
                <Label className="text-sm font-semibold">{t('usageMonitor.displayFieldSection')}</Label>
              </div>
              {isTemplate && (
                <p className="text-xs text-muted-foreground">{t('usageMonitor.templateDisplayFieldsHint')}</p>
              )}
              <DisplayFieldForm
                displayFields={displayFields}
                variables={variables}
                onChange={setDisplayFields}
              />
            </div>

            {/* Test Connection */}
            <TestConnection
              apiUrl={apiUrl}
              apiMethod={apiMethod}
              apiHeaders={apiHeaders}
              apiBody={apiBody}
              fields={testFields}
            />
          </div>
        </div>

        <DialogFooter className="flex-shrink-0">
          <Button type="button" variant="outline" onClick={() => setOpen(null)}>
            {t('common.buttons.cancel')}
          </Button>
          <Button
            type="button"
            onClick={handleSubmit}
            disabled={!canSubmit || updateMutation.isPending}
          >
            {updateMutation.isPending ? t('common.buttons.saving') : t('common.buttons.confirm')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
