'use client';

import React, { useState, useEffect } from 'react';
import { Loader2, Save } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { AutoCompleteSelect } from '@/components/auto-complete-select';
import { useSystemContext } from '../context/system-context';
import { currencyCodes } from '../data/currencies';
import {
  useGeneralSettings,
  useUpdateGeneralSettings,
  useConcurrencyLimitSettings,
  useUpdateConcurrencyLimitSettings,
} from '../data/system';
import { GMTTimeZoneOptions } from '../data/timezones';

export function GeneralSettings() {
  const { t } = useTranslation();
  const { data: settings, isLoading: isLoadingSettings } = useGeneralSettings();
  const updateSettings = useUpdateGeneralSettings();
  const { isLoading, setIsLoading } = useSystemContext();

  // Per-user concurrency limit
  const { data: concurrencyLimitSettings, isLoading: isLoadingConcurrencyLimit } = useConcurrencyLimitSettings();
  const updateConcurrencyLimit = useUpdateConcurrencyLimitSettings();
  const [maxConcurrentRequestsPerUser, setMaxConcurrentRequestsPerUser] = useState('0');

  useEffect(() => {
    if (concurrencyLimitSettings) {
      setMaxConcurrentRequestsPerUser(String(concurrencyLimitSettings.maxConcurrentRequestsPerUser));
    }
  }, [concurrencyLimitSettings]);

  const handleConcurrencyLimitSave = () => {
    const parsed = Number.parseInt(maxConcurrentRequestsPerUser, 10);
    const value = Number.isFinite(parsed) && parsed > 0 ? parsed : 0;
    updateConcurrencyLimit.mutate({ maxConcurrentRequestsPerUser: value });
  };

  const [currencyCode, setCurrencyCode] = useState('USD');
  const [timezone, setTimezone] = useState('UTC');

  const currencyItems = React.useMemo(
    () =>
      currencyCodes.map((code) => ({
        value: code,
        label: t(`currencies.${code}`),
      })),
    [t]
  );

  const timezoneItems = React.useMemo(() => GMTTimeZoneOptions, []);

  // Update local state when settings are loaded
  useEffect(() => {
    if (settings) {
      setCurrencyCode(settings.currencyCode || 'USD');
      setTimezone(settings.timezone || 'UTC');
    }
  }, [settings]);

  const handleSave = async () => {
    setIsLoading(true);
    try {
      await updateSettings.mutateAsync({
        currencyCode: currencyCode.trim(),
        timezone: timezone.trim(),
      });
    } finally {
      setIsLoading(false);
    }
  };

  const hasChanges = settings ? settings.currencyCode !== currencyCode || settings.timezone !== timezone : false;

  if (isLoadingSettings) {
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
          <CardTitle>{t('system.general.title')}</CardTitle>
          <CardDescription>{t('system.general.description')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-6'>
          <div className='space-y-2'>
            <Label htmlFor='currency-code'>{t('system.general.currencyCode.label')}</Label>
            <div className='max-w-md'>
              <AutoCompleteSelect
                selectedValue={currencyCode}
                onSelectedValueChange={setCurrencyCode}
                items={currencyItems}
                placeholder={t('system.general.currencyCode.placeholder')}
                isLoading={isLoadingSettings}
              />
            </div>
            <div className='text-muted-foreground text-sm'>{t('system.general.currencyCode.description')}</div>
          </div>

          <div className='space-y-2'>
            <Label htmlFor='timezone'>{t('system.general.timezone.label')}</Label>
            <div className='max-w-md'>
              <AutoCompleteSelect
                selectedValue={timezone}
                onSelectedValueChange={setTimezone}
                items={timezoneItems}
                placeholder={t('system.general.timezone.placeholder')}
                isLoading={isLoadingSettings}
              />
            </div>
            <div className='text-muted-foreground text-sm'>{t('system.general.timezone.description')}</div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>{t('system.concurrencyLimit.title')}</CardTitle>
          <CardDescription>{t('system.concurrencyLimit.description')}</CardDescription>
        </CardHeader>
        <CardContent className='space-y-4'>
          <div className='space-y-2'>
            <Label htmlFor='max-concurrent-requests'>{t('system.concurrencyLimit.label')}</Label>
            <div className='flex max-w-md items-center gap-2'>
              <Input
                id='max-concurrent-requests'
                type='number'
                min={0}
                step={1}
                value={maxConcurrentRequestsPerUser}
                onChange={(event) => setMaxConcurrentRequestsPerUser(event.target.value)}
                disabled={isLoadingConcurrencyLimit}
              />
              <Button
                onClick={handleConcurrencyLimitSave}
                disabled={isLoadingConcurrencyLimit || updateConcurrencyLimit.isPending}
                className='min-w-[100px]'
              >
                {updateConcurrencyLimit.isPending ? (
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
            <div className='text-muted-foreground text-sm'>{t('system.concurrencyLimit.helpText')}</div>
          </div>
        </CardContent>
      </Card>

      {hasChanges && (
        <div className='flex justify-end'>
          <Button onClick={handleSave} disabled={isLoading || updateSettings.isPending} className='min-w-[100px]'>
            {isLoading || updateSettings.isPending ? (
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
      )}
    </div>
  );
}
