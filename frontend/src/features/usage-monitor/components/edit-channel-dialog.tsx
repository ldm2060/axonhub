'use client';

import { useState, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { ScrollArea } from '@/components/ui/scroll-area';
import { Eye, EyeOff } from 'lucide-react';
import { useUpdateUsageMonitorChannel } from '../data/usage-monitor';
import { useUsageMonitorContext } from '../context/usage-monitor-context';
import type { FieldConfig } from '../data/schema';
import { FieldConfigForm } from './field-config-form';
import { TestConnection } from './test-connection';

const MASKED_API_KEY = '••••••••';

export function EditChannelDialog() {
  const { t } = useTranslation();
  const { open, setOpen, currentChannel } = useUsageMonitorContext();
  const updateMutation = useUpdateUsageMonitorChannel();

  const isOpen = open === 'edit';

  const isTemplate = currentChannel?.source === 'template';

  const [name, setName] = useState('');
  const [apiUrl, setApiUrl] = useState('');
  const [apiMethod, setApiMethod] = useState<'GET' | 'POST'>('GET');
  const [apiHeaders, setApiHeaders] = useState('');
  const [apiBody, setApiBody] = useState('');
  const [pollIntervalMin, setPollIntervalMin] = useState(5);
  const [fields, setFields] = useState<FieldConfig[]>([]);
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
    setFields(currentChannel.fields ?? []);
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
        // Template channels: only send name, apiKey (if changed), and pollInterval
        await updateMutation.mutateAsync({
          id: currentChannel.id,
          input: {
            name,
            pollInterval: pollIntervalMin * 60,
            ...(apiKey.trim() && apiKey !== MASKED_API_KEY ? { apiKey } : {}),
          },
        });
      } else {
        // Non-template channels: send everything as before
        await updateMutation.mutateAsync({
          id: currentChannel.id,
          input: {
            name,
            apiUrl,
            apiMethod,
            apiHeaders,
            apiBody: apiBody || undefined,
            pollInterval: pollIntervalMin * 60,
            fields,
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
    : name.trim() && apiUrl.trim() && !headersError;

  return (
    <Dialog
      open={isOpen}
      onOpenChange={(v) => {
        if (!v) setOpen(null);
      }}
    >
      <DialogContent className="flex max-h-[90vh] flex-col sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t('usageMonitor.editChannel')}</DialogTitle>
          <DialogDescription />
        </DialogHeader>

        <ScrollArea className="min-h-0 flex-1 pr-2">
          <div className="space-y-5 pb-4">
            {/* Template channel: API Key for rotation */}
            {isTemplate && (
              <div className="space-y-3">
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
                    {apiKey === MASKED_API_KEY ? 'Leave unchanged to keep current key' : ''}
                  </p>
                </div>
              </div>
            )}

            {/* Non-template: API URL */}
            {!isTemplate && (
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

            {/* Non-template: API Method */}
            {!isTemplate && (
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

            {/* Non-template: API Headers */}
            {!isTemplate && (
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

            {/* Non-template: API Body */}
            {!isTemplate && apiMethod === 'POST' && (
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
              <p className="text-xs text-muted-foreground">minutes</p>
            </div>

            {/* Non-template: Field Configs */}
            {!isTemplate && (
              <FieldConfigForm fields={fields} onChange={setFields} />
            )}

            {/* Non-template: Test Connection */}
            {!isTemplate && (
              <TestConnection
                apiUrl={apiUrl}
                apiMethod={apiMethod}
                apiHeaders={apiHeaders}
                apiBody={apiBody}
                fields={fields}
              />
            )}
          </div>
        </ScrollArea>

        <DialogFooter>
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
