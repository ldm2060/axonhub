'use client';

import { useEffect, useMemo, useState } from 'react';
import { AlertTriangle, CheckCircle2, KeyRound, Loader2, Save } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { usePermissions } from '@/hooks/usePermissions';
import { Alert, AlertDescription, AlertTitle } from '@/components/ui/alert';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Switch } from '@/components/ui/switch';
import { useTurnstileSettings, useUpdateTurnstileSettings } from '../data/system';

export function TurnstileSettingsCard() {
  const { t } = useTranslation();
  const { hasSystemScope } = usePermissions();
  const canWrite = hasSystemScope('write_settings');
  const settingsQuery = useTurnstileSettings();
  const updateSettings = useUpdateTurnstileSettings();
  const [enabled, setEnabled] = useState(false);
  const [siteKey, setSiteKey] = useState('');
  const [secretKey, setSecretKey] = useState('');

  useEffect(() => {
    if (settingsQuery.data) {
      setEnabled(settingsQuery.data.enabled);
      setSiteKey(settingsQuery.data.siteKey);
      setSecretKey('');
    }
  }, [settingsQuery.data]);

  const trimmedSiteKey = siteKey.trim();
  const hasChanges = useMemo(() => {
    const settings = settingsQuery.data;
    if (!settings) return false;

    return enabled !== settings.enabled || trimmedSiteKey !== settings.siteKey || secretKey.trim().length > 0;
  }, [enabled, secretKey, settingsQuery.data, trimmedSiteKey]);

  const canSave =
    canWrite &&
    hasChanges &&
    !updateSettings.isPending &&
    (!enabled || (trimmedSiteKey.length > 0 && (settingsQuery.data?.secretConfigured || secretKey.trim().length > 0)));

  const handleSave = async () => {
    const replacementSecret = secretKey.trim();
    await updateSettings.mutateAsync({
      enabled,
      siteKey: trimmedSiteKey,
      ...(replacementSecret ? { secretKey: replacementSecret } : {}),
    });
    setSecretKey('');
  };

  if (settingsQuery.isLoading) {
    return (
      <Card>
        <CardContent className='flex h-32 items-center justify-center'>
          <Loader2 className='h-6 w-6 animate-spin' />
          <span className='text-muted-foreground ml-2'>{t('common.loading')}</span>
        </CardContent>
      </Card>
    );
  }

  if (settingsQuery.isError || !settingsQuery.data) {
    return (
      <Card>
        <CardHeader>
          <CardTitle>{t('system.turnstile.title')}</CardTitle>
          <CardDescription>{t('system.turnstile.description')}</CardDescription>
        </CardHeader>
        <CardContent>
          <Alert variant='destructive'>
            <AlertTriangle className='h-4 w-4' />
            <AlertTitle>{t('system.turnstile.loadErrorTitle')}</AlertTitle>
            <AlertDescription className='flex flex-wrap items-center justify-between gap-3'>
              <span>{t('system.turnstile.loadError')}</span>
              <Button type='button' variant='outline' size='sm' onClick={() => void settingsQuery.refetch()}>
                {t('common.buttons.retry')}
              </Button>
            </AlertDescription>
          </Alert>
        </CardContent>
      </Card>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('system.turnstile.title')}</CardTitle>
        <CardDescription>{t('system.turnstile.description')}</CardDescription>
      </CardHeader>
      <CardContent className='space-y-6'>
        <Alert>
          <AlertTriangle className='h-4 w-4' />
          <AlertTitle>{t('system.turnstile.warningTitle')}</AlertTitle>
          <AlertDescription>{t('system.turnstile.warning')}</AlertDescription>
        </Alert>

        <div className='flex items-center justify-between gap-4 rounded-lg border p-4'>
          <div className='space-y-1'>
            <Label htmlFor='turnstile-enabled'>{t('system.turnstile.enabled.label')}</Label>
            <div className='text-muted-foreground text-sm'>{t('system.turnstile.enabled.description')}</div>
          </div>
          <Switch
            id='turnstile-enabled'
            checked={enabled}
            onCheckedChange={setEnabled}
            disabled={!canWrite || updateSettings.isPending}
            data-testid='turnstile-settings-enabled'
          />
        </div>

        <div className='space-y-2'>
          <Label htmlFor='turnstile-site-key'>{t('system.turnstile.siteKey.label')}</Label>
          <Input
            id='turnstile-site-key'
            value={siteKey}
            onChange={(event) => setSiteKey(event.target.value)}
            placeholder={t('system.turnstile.siteKey.placeholder')}
            disabled={!canWrite || updateSettings.isPending}
            autoComplete='off'
            data-testid='turnstile-settings-site-key'
          />
          <div className='text-muted-foreground text-sm'>{t('system.turnstile.siteKey.description')}</div>
        </div>

        <div className='space-y-2'>
          <div className='flex flex-wrap items-center justify-between gap-2'>
            <Label htmlFor='turnstile-secret-key'>{t('system.turnstile.secretKey.label')}</Label>
            <span className='flex items-center gap-1 text-sm'>
              {settingsQuery.data.secretConfigured ? (
                <>
                  <CheckCircle2 className='h-4 w-4 text-emerald-600' />
                  {t('system.turnstile.secretKey.configured')}
                </>
              ) : (
                <>
                  <KeyRound className='text-muted-foreground h-4 w-4' />
                  {t('system.turnstile.secretKey.notConfigured')}
                </>
              )}
            </span>
          </div>
          <Input
            id='turnstile-secret-key'
            type='password'
            value={secretKey}
            onChange={(event) => setSecretKey(event.target.value)}
            placeholder={
              settingsQuery.data.secretConfigured
                ? t('system.turnstile.secretKey.replacePlaceholder')
                : t('system.turnstile.secretKey.placeholder')
            }
            disabled={!canWrite || updateSettings.isPending}
            autoComplete='new-password'
            data-testid='turnstile-settings-secret-key'
          />
          <div className='text-muted-foreground text-sm'>{t('system.turnstile.secretKey.description')}</div>
        </div>

        {!canWrite && <div className='text-muted-foreground text-sm'>{t('system.turnstile.readOnly')}</div>}

        <div className='flex justify-end'>
          <Button onClick={handleSave} disabled={!canSave} className='min-w-[100px]' data-testid='turnstile-settings-save'>
            {updateSettings.isPending ? (
              <>
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                {t('system.buttons.saving')}
              </>
            ) : (
              <>
                <Save className='mr-2 h-4 w-4' />
                {t('system.buttons.save')}
              </>
            )}
          </Button>
        </div>
      </CardContent>
    </Card>
  );
}
