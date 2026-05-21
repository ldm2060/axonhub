'use client';

import React, { useState, useEffect } from 'react';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Badge } from '@/components/ui/badge';
import { Separator } from '@/components/ui/separator';
import { useRegistrationSettings, useUpdateRegistrationSettings } from '../data/registration-email-settings';

export function RegistrationSettingsTab() {
  const { t } = useTranslation();
  const { data: settings, isLoading } = useRegistrationSettings();
  const updateSettings = useUpdateRegistrationSettings();

  const [enabled, setEnabled] = useState(false);
  const [mode, setMode] = useState('auto');
  const [defaultScopes, setDefaultScopes] = useState<string[]>([]);

  useEffect(() => {
    if (settings) {
      setEnabled(settings.enabled);
      setMode(settings.mode || 'auto');
      setDefaultScopes(settings.defaultScopes || []);
    }
  }, [settings]);

  const handleEnabledChange = async (checked: boolean) => {
    const previousValue = enabled;
    setEnabled(checked);
    try {
      await updateSettings.mutateAsync({ enabled: checked });
    } catch {
      setEnabled(previousValue);
    }
  };

  const handleModeChange = async (newMode: string) => {
    const previousValue = mode;
    setMode(newMode);
    try {
      await updateSettings.mutateAsync({ mode: newMode });
    } catch {
      setMode(previousValue);
    }
  };

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
          <div className='flex items-center justify-between'>
            <div className='space-y-0.5'>
              <Label htmlFor='registration-enabled'>{t('system.registration.enabled.label')}</Label>
              <div className='text-muted-foreground text-sm'>{t('system.registration.enabled.description')}</div>
            </div>
            <Switch
              id='registration-enabled'
              checked={enabled}
              onCheckedChange={handleEnabledChange}
              disabled={updateSettings.isPending}
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
            </>
          )}
        </CardContent>
      </Card>
    </div>
  );
}
