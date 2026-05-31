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
import { useQueryChannels } from '@/features/channels/data/channels';
import { useCreateUsageMonitorChannel } from '../data/usage-monitor';
import { useUsageMonitorContext } from '../context/usage-monitor-context';
import type { FieldConfig } from '../data/schema';
import { FieldConfigForm } from './field-config-form';
import { TestConnection } from './test-connection';

type SourceType = 'builtin' | 'custom';

export function AddChannelDialog() {
  const { t } = useTranslation();
  const { open, setOpen } = useUsageMonitorContext();
  const createMutation = useCreateUsageMonitorChannel();
  const channelsQuery = useQueryChannels({ first: 200, where: { statusIn: ['enabled'] } });

  const isOpen = open === 'add';

  const [source, setSource] = useState<SourceType>('custom');
  const [channelId, setChannelId] = useState('');
  const [name, setName] = useState('');
  const [apiUrl, setApiUrl] = useState('');
  const [apiMethod, setApiMethod] = useState<'GET' | 'POST'>('GET');
  const [apiHeaders, setApiHeaders] = useState('');
  const [apiBody, setApiBody] = useState('');
  const [pollIntervalMin, setPollIntervalMin] = useState(5);
  const [fields, setFields] = useState<FieldConfig[]>([]);
  const [headersError, setHeadersError] = useState('');

  // Auto-fill from selected channel
  useEffect(() => {
    if (source !== 'builtin' || !channelId) return;
    const channels = channelsQuery.data?.edges ?? [];
    const selected = channels.find((e) => e.node.id === channelId);
    if (selected) {
      setApiUrl(selected.node.baseURL || '');
      // Build headers from credentials
      const creds = selected.node.credentials;
      const headers: Record<string, string> = {};
      if (creds?.apiKey) {
        headers['Authorization'] = `Bearer ${creds.apiKey}`;
      } else if (creds?.apiKeys && creds.apiKeys.length > 0) {
        headers['Authorization'] = `Bearer ${creds.apiKeys[0]}`;
      }
      setApiHeaders(Object.keys(headers).length > 0 ? JSON.stringify(headers, null, 2) : '');
      if (!name) {
        setName(selected.node.name);
      }
    }
  }, [source, channelId, channelsQuery.data, name]);

  // Validate JSON headers
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

  function resetForm() {
    setSource('custom');
    setChannelId('');
    setName('');
    setApiUrl('');
    setApiMethod('GET');
    setApiHeaders('');
    setApiBody('');
    setPollIntervalMin(5);
    setFields([]);
    setHeadersError('');
  }

  async function handleSubmit() {
    try {
      await createMutation.mutateAsync({
        name,
        source,
        channelId: source === 'builtin' ? channelId : undefined,
        apiUrl,
        apiMethod,
        apiHeaders,
        apiBody: apiBody || undefined,
        pollInterval: pollIntervalMin * 60,
        fields,
      });
      setOpen(null);
      resetForm();
    } catch {
      // error handled by mutation
    }
  }

  const canSubmit = name.trim() && apiUrl.trim() && !headersError;

  return (
    <Dialog
      open={isOpen}
      onOpenChange={(v) => {
        if (!v) {
          setOpen(null);
          resetForm();
        }
      }}
    >
      <DialogContent className="flex max-h-[90vh] flex-col sm:max-w-2xl">
        <DialogHeader>
          <DialogTitle>{t('usageMonitor.addChannel')}</DialogTitle>
          <DialogDescription />
        </DialogHeader>

        <ScrollArea className="min-h-0 flex-1 pr-2">
          <div className="space-y-5 pb-4">
            {/* Source Selection */}
            <div className="space-y-2">
              <Label>{t('usageMonitor.source.builtin')}</Label>
              <div className="grid grid-cols-2 gap-3">
                <button
                  type="button"
                  onClick={() => setSource('builtin')}
                  className={`rounded-lg border-2 p-3 text-center text-sm font-medium transition-colors ${
                    source === 'builtin'
                      ? 'border-primary bg-primary/5 text-primary'
                      : 'border-muted hover:border-muted-foreground/30'
                  }`}
                >
                  {t('usageMonitor.source.builtin')}
                </button>
                <button
                  type="button"
                  onClick={() => setSource('custom')}
                  className={`rounded-lg border-2 p-3 text-center text-sm font-medium transition-colors ${
                    source === 'custom'
                      ? 'border-primary bg-primary/5 text-primary'
                      : 'border-muted hover:border-muted-foreground/30'
                  }`}
                >
                  {t('usageMonitor.source.custom')}
                </button>
              </div>
            </div>

            {/* Channel Selector (builtin) */}
            {source === 'builtin' && (
              <div className="space-y-1.5">
                <Label>{t('usageMonitor.selectChannel')}</Label>
                <Select value={channelId} onValueChange={setChannelId}>
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder={t('usageMonitor.selectChannel')} />
                  </SelectTrigger>
                  <SelectContent>
                    {(channelsQuery.data?.edges ?? []).map((edge) => (
                      <SelectItem key={edge.node.id} value={edge.node.id}>
                        {edge.node.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            )}

            {/* Custom API Config */}
            {source === 'custom' && (
              <div className="space-y-3">
                <div className="space-y-1.5">
                  <Label>{t('usageMonitor.apiUrl')}</Label>
                  <Input
                    value={apiUrl}
                    onChange={(e) => setApiUrl(e.target.value)}
                    placeholder="https://api.example.com/v1/usage"
                    className="font-mono"
                  />
                </div>

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

                {apiMethod === 'POST' && (
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

            {/* Field Configs */}
            <FieldConfigForm fields={fields} onChange={setFields} />

            {/* Test Connection */}
            <TestConnection
              apiUrl={apiUrl}
              apiMethod={apiMethod}
              apiHeaders={apiHeaders}
              apiBody={apiBody}
              fields={fields}
            />
          </div>
        </ScrollArea>

        <DialogFooter>
          <Button type="button" variant="outline" onClick={() => { setOpen(null); resetForm(); }}>
            {t('common.buttons.cancel')}
          </Button>
          <Button
            type="button"
            onClick={handleSubmit}
            disabled={!canSubmit || createMutation.isPending}
          >
            {createMutation.isPending ? t('common.buttons.saving') : t('common.buttons.confirm')}
          </Button>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}
