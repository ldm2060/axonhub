'use client';

import React, { useState, useEffect, useCallback } from 'react';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Separator } from '@/components/ui/separator';
import { Switch } from '@/components/ui/switch';
import { Textarea } from '@/components/ui/textarea';
import { useRegistrationSettings, useUpdateRegistrationSettings, useEmailSettings } from '../data/registration-email-settings';

function isValidRegex(pattern: string): boolean {
  try {
    new RegExp(pattern);
    return true;
  } catch {
    return false;
  }
}

export function RegistrationSettingsTab() {
  const { t } = useTranslation();
  const { data: settings, isLoading } = useRegistrationSettings();
  const { data: emailSettings } = useEmailSettings();
  const updateSettings = useUpdateRegistrationSettings();

  const emailConfigured = !!(emailSettings && emailSettings.smtpHost && emailSettings.smtpPort > 0 && emailSettings.fromAddress);

  const [enabled, setEnabled] = useState(false);
  const [mode, setMode] = useState('auto');
  const [defaultScopes, setDefaultScopes] = useState<string[]>([]);
  const [allowPatterns, setAllowPatterns] = useState('');
  const [denyPatterns, setDenyPatterns] = useState('');
  const [patternsDirty, setPatternsDirty] = useState(false);

  useEffect(() => {
    if (settings) {
      setEnabled(settings.allowSignUp);
      setMode(settings.approvalRequired ? 'approval' : 'auto');
      setDefaultScopes(settings.defaultUserScopes || []);
      setAllowPatterns((settings.emailAllowPatterns || []).join('\n'));
      setDenyPatterns((settings.emailDenyPatterns || []).join('\n'));
      setPatternsDirty(false);
    }
  }, [settings]);

  const handleEnabledChange = async (checked: boolean) => {
    const previousValue = enabled;
    setEnabled(checked);
    try {
      await updateSettings.mutateAsync({
        allowSignUp: checked,
        approvalRequired: mode === 'approval',
        defaultUserScopes: defaultScopes,
        emailAllowPatterns: allowPatterns
          .split('\n')
          .map((l) => l.trim())
          .filter(Boolean),
        emailDenyPatterns: denyPatterns
          .split('\n')
          .map((l) => l.trim())
          .filter(Boolean),
      });
    } catch (err: any) {
      setEnabled(previousValue);
      const msg = String(err?.message || err || '');
      if (msg.includes('email is not configured')) {
        toast.error(t('system.registration.enabled.emailRequired'));
      }
    }
  };

  const handleModeChange = async (newMode: string) => {
    const previousValue = mode;
    setMode(newMode);
    try {
      await updateSettings.mutateAsync({
        allowSignUp: enabled,
        approvalRequired: newMode === 'approval',
        defaultUserScopes: defaultScopes,
        emailAllowPatterns: allowPatterns
          .split('\n')
          .map((l) => l.trim())
          .filter(Boolean),
        emailDenyPatterns: denyPatterns
          .split('\n')
          .map((l) => l.trim())
          .filter(Boolean),
      });
    } catch {
      setMode(previousValue);
    }
  };

  const handleSavePatterns = useCallback(async () => {
    const allow = allowPatterns
      .split('\n')
      .map((l) => l.trim())
      .filter(Boolean);
    const deny = denyPatterns
      .split('\n')
      .map((l) => l.trim())
      .filter(Boolean);

    const invalidAllow = allow.find((p) => !isValidRegex(p));
    if (invalidAllow) {
      toast.error(t('system.registration.patterns.invalidRegex', { pattern: invalidAllow }));
      return;
    }
    const invalidDeny = deny.find((p) => !isValidRegex(p));
    if (invalidDeny) {
      toast.error(t('system.registration.patterns.invalidRegex', { pattern: invalidDeny }));
      return;
    }

    try {
      await updateSettings.mutateAsync({
        allowSignUp: enabled,
        approvalRequired: mode === 'approval',
        defaultUserScopes: defaultScopes,
        emailAllowPatterns: allow,
        emailDenyPatterns: deny,
      });
      setPatternsDirty(false);
    } catch {
      toast.error(t('common.errors.systemUpdateFailed'));
    }
  }, [allowPatterns, denyPatterns, enabled, mode, defaultScopes, updateSettings, t]);

  if (isLoading) {
    return (
      <div className='flex h-32 items-center justify-center'>
        <Loader2 className='h-6 w-6 animate-spin' />
        <span className='text-muted-foreground ml-2'>{t('common.loading')}</span>
      </div>
    );
  }

  return (
    <div className='space-y-6'>
      <Card>
        <CardHeader>
          <CardTitle>{t('system.registration.title')}</CardTitle>
          <CardDescription>{t('system.registration.description')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-6'>
          {!emailConfigured && !enabled && (
            <div className='rounded-md border border-amber-200 bg-amber-50 px-3 py-2 text-sm text-amber-800 dark:border-amber-900 dark:bg-amber-950/30 dark:text-amber-200'>
              {t('system.registration.enabled.emailRequired')}
            </div>
          )}
          <div className='flex items-center justify-between'>
            <div className='space-y-0.5'>
              <Label htmlFor='registration-enabled'>{t('system.registration.enabled.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.registration.enabled.description')}</div>
            </div>
            <Switch
              id='registration-enabled'
              checked={enabled}
              onCheckedChange={handleEnabledChange}
              disabled={updateSettings.isPending || (!emailConfigured && !enabled)}
            />
          </div>

          {enabled && (
            <>
              <Separator />

              <div className='space-y-2'>
                <Label htmlFor='registration-mode'>{t('system.registration.mode.label')}</Label>
                <div className='text-muted-foreground mb-2 text-sm'>{t('system.registration.mode.description')}</div>
                <Select value={mode} onValueChange={handleModeChange}>
                  <SelectTrigger id='registration-mode' className='w-64'>
                    <SelectValue placeholder={t('system.registration.mode.placeholder')} />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value='auto'>{t('system.registration.mode.options.auto')}</SelectItem>
                    <SelectItem value='approval'>{t('system.registration.mode.options.approval')}</SelectItem>
                  </SelectContent>
                </Select>
              </div>

              <Separator />

              <div className='space-y-2'>
                <Label>{t('system.registration.defaultScopes.label')}</Label>
                <div className='text-muted-foreground text-sm'>{t('system.registration.defaultScopes.description')}</div>
                {defaultScopes.length > 0 ? (
                  <div className='mt-2 flex flex-wrap gap-2'>
                    {defaultScopes.map((scope) => (
                      <Badge key={scope} variant='secondary' className='text-xs'>
                        {t(`scopes.${scope}`, scope)}
                      </Badge>
                    ))}
                  </div>
                ) : (
                  <div className='text-muted-foreground mt-1 text-sm'>{t('system.registration.defaultScopes.empty')}</div>
                )}
              </div>

              <Separator />

              <div className='space-y-4'>
                <div>
                  <h4 className='text-sm font-medium'>{t('system.registration.patterns.title')}</h4>
                  <p className='text-muted-foreground text-sm'>{t('system.registration.patterns.description')}</p>
                </div>

                <div className='grid grid-cols-2 gap-4'>
                  <div className='space-y-2'>
                    <Label htmlFor='allow-patterns'>{t('system.registration.patterns.allow.label')}</Label>
                    <p className='text-muted-foreground text-xs'>{t('system.registration.patterns.allow.description')}</p>
                    <Textarea
                      id='allow-patterns'
                      value={allowPatterns}
                      onChange={(e) => {
                        setAllowPatterns(e.target.value);
                        setPatternsDirty(true);
                      }}
                      placeholder={t('system.registration.patterns.placeholder')}
                      rows={5}
                      className='font-mono text-sm'
                    />
                  </div>

                  <div className='space-y-2'>
                    <Label htmlFor='deny-patterns'>{t('system.registration.patterns.deny.label')}</Label>
                    <p className='text-muted-foreground text-xs'>{t('system.registration.patterns.deny.description')}</p>
                    <Textarea
                      id='deny-patterns'
                      value={denyPatterns}
                      onChange={(e) => {
                        setDenyPatterns(e.target.value);
                        setPatternsDirty(true);
                      }}
                      placeholder={t('system.registration.patterns.placeholder')}
                      rows={5}
                      className='font-mono text-sm'
                    />
                  </div>
                </div>

                {patternsDirty && (
                  <div className='flex justify-end'>
                    <Button onClick={handleSavePatterns} disabled={updateSettings.isPending}>
                      {updateSettings.isPending ? (
                        <>
                          <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                          {t('system.buttons.saving')}
                        </>
                      ) : (
                        t('system.buttons.save')
                      )}
                    </Button>
                  </div>
                )}
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
