'use client';

import { useState, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Textarea } from '@/components/ui/textarea';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { useQueryChannels } from '@/features/channels/data/channels';
import { useCreateUsageMonitorChannel } from '../data/usage-monitor';
import { useQuotaMonitorTemplates, type QuotaMonitorTemplate } from '../data/templates';
import { useUsageMonitorContext } from '../context/usage-monitor-context';
import type { FieldConfig, Variable, DisplayField, VariableInput, DisplayFieldInput } from '../data/schema';
import { VariableForm } from './variable-form';
import { DisplayFieldForm } from './display-field-form';
import { TestConnection } from './test-connection';
import { Eye, EyeOff, Lock } from 'lucide-react';

type SourceType = 'builtin' | 'custom' | 'template';

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
  const [variables, setVariables] = useState<Variable[]>([]);
  const [displayFields, setDisplayFields] = useState<DisplayField[]>([]);
  const [headersError, setHeadersError] = useState('');

  const templates = useQuotaMonitorTemplates();
  const [selectedTemplate, setSelectedTemplate] = useState<QuotaMonitorTemplate | null>(null);
  const [apiKey, setApiKey] = useState('');
  const [showApiKey, setShowApiKey] = useState(false);

  // Reset form fields when switching tabs
  const resetForSource = useCallback((newSource: SourceType) => {
    setSource(newSource);
    setChannelId('');
    setName('');
    setApiUrl('');
    setApiMethod('GET');
    setApiHeaders('');
    setApiBody('');
    setPollIntervalMin(5);
    setVariables([]);
    setDisplayFields([]);
    setHeadersError('');
    setSelectedTemplate(null);
    setApiKey('');
    setShowApiKey(false);
  }, []);

  // Auto-fill from selected channel (builtin)
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

  function handleTemplateChange(providerType: string) {
    const tmpl = templates.find((t) => t.providerType === providerType);
    setSelectedTemplate(tmpl ?? null);
    if (tmpl) {
      setApiUrl(tmpl.apiUrl);
      setApiMethod(tmpl.apiMethod);
      if (tmpl.apiBody) setApiBody(tmpl.apiBody);
      else setApiBody('');
      if (!name) setName(tmpl.name);
      // Populate variables and display fields from template
      setVariables(tmpl.variables ?? []);
      setDisplayFields(tmpl.displayFields ?? []);
    } else {
      setVariables([]);
      setDisplayFields([]);
    }
  }

  function resetForm() {
    resetForSource('custom');
  }

  async function handleSubmit() {
    try {
      // Convert variables and displayFields to input types
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

      // For template source, use template fields as the legacy fields array
      const legacyFields: FieldConfig[] = source === 'template'
        ? (selectedTemplate?.fields ?? [])
        : [];

      await createMutation.mutateAsync({
        name,
        source,
        channelId: source === 'builtin' ? channelId : undefined,
        providerType: source === 'template' ? selectedTemplate?.providerType : undefined,
        apiKey: source === 'template' ? apiKey : undefined,
        apiUrl,
        apiMethod,
        apiHeaders: source === 'template' ? '' : apiHeaders,
        apiBody: apiBody || undefined,
        pollInterval: pollIntervalMin * 60,
        fields: source === 'template' ? legacyFields : [],
        variables: variableInputs,
        displayFields: displayFieldInputs,
      });
      setOpen(null);
      resetForm();
    } catch {
      // error handled by mutation
    }
  }

  const canSubmit = name.trim() && apiUrl.trim() && !headersError && (source === 'template' ? (selectedTemplate !== null && apiKey.trim() !== '') : true);

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
        <DialogHeader className="flex-shrink-0">
          <DialogTitle>{t('usageMonitor.addChannel')}</DialogTitle>
          <DialogDescription />
        </DialogHeader>

        <div className="flex-1 min-h-0 overflow-y-auto pr-1">
          <Tabs value={source} onValueChange={(v) => resetForSource(v as SourceType)} className="gap-4">
            <TabsList className="w-full">
              <TabsTrigger value="builtin" className="flex-1">{t('usageMonitor.tabs.builtin')}</TabsTrigger>
              <TabsTrigger value="custom" className="flex-1">{t('usageMonitor.tabs.custom')}</TabsTrigger>
              <TabsTrigger value="template" className="flex-1">{t('usageMonitor.tabs.template')}</TabsTrigger>
            </TabsList>

            {/* ========== Builtin Tab ========== */}
            <TabsContent value="builtin" className="space-y-5 pb-4 mt-0">
              {/* Channel Selector */}
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
                </div>
                <VariableForm variables={variables} onChange={setVariables} />
              </div>

              {/* Display Field Section */}
              <div className="space-y-3">
                <div className="flex items-center gap-2">
                  <Label className="text-sm font-semibold">{t('usageMonitor.displayFieldSection')}</Label>
                </div>
                <DisplayFieldForm displayFields={displayFields} variables={variables} onChange={setDisplayFields} />
              </div>

              {/* Test Connection */}
              <TestConnection
                apiUrl={apiUrl}
                apiMethod={apiMethod}
                apiHeaders={apiHeaders}
                apiBody={apiBody}
                variables={variables}
                displayFields={displayFields}
              />
            </TabsContent>

            {/* ========== Custom Tab ========== */}
            <TabsContent value="custom" className="space-y-5 pb-4 mt-0">
              {/* API URL */}
              <div className="space-y-1.5">
                <Label>{t('usageMonitor.apiUrl')}</Label>
                <Input
                  value={apiUrl}
                  onChange={(e) => setApiUrl(e.target.value)}
                  placeholder="https://api.example.com/v1/usage"
                  className="font-mono"
                />
              </div>

              {/* Method */}
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

              {/* Headers */}
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

              {/* Body (POST only) */}
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
                </div>
                <VariableForm variables={variables} onChange={setVariables} />
              </div>

              {/* Display Field Section */}
              <div className="space-y-3">
                <div className="flex items-center gap-2">
                  <Label className="text-sm font-semibold">{t('usageMonitor.displayFieldSection')}</Label>
                </div>
                <DisplayFieldForm displayFields={displayFields} variables={variables} onChange={setDisplayFields} />
              </div>

              {/* Test Connection */}
              <TestConnection
                apiUrl={apiUrl}
                apiMethod={apiMethod}
                apiHeaders={apiHeaders}
                apiBody={apiBody}
                variables={variables}
                displayFields={displayFields}
              />
            </TabsContent>

            {/* ========== Template Tab ========== */}
            <TabsContent value="template" className="space-y-5 pb-4 mt-0">
              {/* Template Selector */}
              <div className="space-y-1.5">
                <Label>{t('usageMonitor.selectTemplate')}</Label>
                <Select value={selectedTemplate?.providerType ?? ''} onValueChange={handleTemplateChange}>
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder={t('usageMonitor.selectTemplate')} />
                  </SelectTrigger>
                  <SelectContent>
                    {templates.map((tmpl) => (
                      <SelectItem key={tmpl.providerType} value={tmpl.providerType}>
                        {tmpl.name}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>

              {selectedTemplate && (
                <>
                  {selectedTemplate.description && (
                    <p className="text-xs text-muted-foreground">{selectedTemplate.description}</p>
                  )}

                  {/* Credential (API Key / Token) */}
                  <div className="space-y-1.5">
                    <Label>{selectedTemplate.credentialLabel || t('usageMonitor.apiKey')}</Label>
                    <div className="relative">
                      <Input
                        type={showApiKey ? 'text' : 'password'}
                        value={apiKey}
                        onChange={(e) => setApiKey(e.target.value)}
                        placeholder={selectedTemplate.credentialPlaceholder || t('usageMonitor.apiKeyPlaceholder')}
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
                  </div>

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

                  {/* Variable Section (readOnly) */}
                  <div className="space-y-3">
                    <div className="flex items-center gap-2">
                      <Label className="text-sm font-semibold">{t('usageMonitor.variableSection')}</Label>
                      <Lock className="size-3.5 text-muted-foreground" />
                    </div>
                    <p className="text-xs text-muted-foreground">{t('usageMonitor.templateVariablesHint')}</p>
                    <VariableForm variables={variables} onChange={setVariables} readOnly />
                  </div>

                  {/* Display Field Section (editable) */}
                  <div className="space-y-3">
                    <div className="flex items-center gap-2">
                      <Label className="text-sm font-semibold">{t('usageMonitor.displayFieldSection')}</Label>
                    </div>
                    <p className="text-xs text-muted-foreground">{t('usageMonitor.templateDisplayFieldsHint')}</p>
                    <DisplayFieldForm displayFields={displayFields} variables={variables} onChange={setDisplayFields} />
                  </div>

                  {/* Test Connection */}
                  <TestConnection
                    apiUrl={apiUrl}
                    apiMethod={apiMethod}
                    apiHeaders={''}
                    apiBody={apiBody}
                    variables={variables}
                    displayFields={displayFields}
                  />
                </>
              )}
            </TabsContent>
          </Tabs>
        </div>

        <DialogFooter className="flex-shrink-0">
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
