'use client';

import React, { useState, useEffect } from 'react';
import { Loader2, Mail, Send } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Separator } from '@/components/ui/separator';
import { RadioGroup, RadioGroupItem } from '@/components/ui/radio-group';
import { useEmailSettings, useUpdateEmailSettings, useTestEmailConnection } from '../data/registration-email-settings';

export function EmailSettingsTab() {
  const { t } = useTranslation();
  const { data: settings, isLoading } = useEmailSettings();
  const updateSettings = useUpdateEmailSettings();
  const testConnection = useTestEmailConnection();

  const [formData, setFormData] = useState({
    smtpHost: '',
    smtpPort: 587,
    smtpUser: '',
    smtpPassword: '',
    encryption: 'STARTTLS',
    fromName: '',
    fromAddress: '',
  });

  const [passwordChanged, setPasswordChanged] = useState(false);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    if (settings) {
      setFormData({
        smtpHost: settings.smtpHost || '',
        smtpPort: settings.smtpPort || 587,
        smtpUser: settings.smtpUser || '',
        smtpPassword: settings.smtpPassword ? '********' : '',
        encryption: settings.encryption || 'STARTTLS',
        fromName: settings.fromName || '',
        fromAddress: settings.fromAddress || '',
      });
      setConnected(settings.connected || false);
      setPasswordChanged(false);
    }
  }, [settings]);

  const handleInputChange = (field: string, value: string | number) => {
    setFormData((prev) => ({ ...prev, [field]: value }));
    if (field === 'smtpPassword') {
      setPasswordChanged(true);
    }
  };

  const hasChanges = settings
    ? formData.smtpHost !== (settings.smtpHost || '') ||
      formData.smtpPort !== (settings.smtpPort || 587) ||
      formData.smtpUser !== (settings.smtpUser || '') ||
      passwordChanged ||
      formData.encryption !== (settings.encryption || 'STARTTLS') ||
      formData.fromName !== (settings.fromName || '') ||
      formData.fromAddress !== (settings.fromAddress || '')
    : false;

  const handleSave = async () => {
    const input: Record<string, unknown> = {
      smtpHost: formData.smtpHost,
      smtpPort: formData.smtpPort,
      smtpUser: formData.smtpUser,
      encryption: formData.encryption,
      fromName: formData.fromName,
      fromAddress: formData.fromAddress,
      smtpPassword: passwordChanged ? formData.smtpPassword : (settings?.smtpPassword || ''),
    };

    const result = await updateSettings.mutateAsync(input as any);
    if (result) {
      setPasswordChanged(false);
    }
  };

  const handleTestConnection = async () => {
    try {
      const result = await testConnection.mutateAsync();
      setConnected(result.success);
    } catch {
      setConnected(false);
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
          <div className='flex items-center justify-between'>
            <div>
              <CardTitle className='flex items-center gap-2'>
                <Mail className='h-5 w-5' />
                {t('system.email.title')}
              </CardTitle>
              <CardDescription>{t('system.email.description')}</CardDescription>
            </div>
            <div className='flex items-center gap-2'>
              <div className={`h-3 w-3 rounded-full ${connected ? 'bg-green-500' : 'bg-red-500'}`} title={connected ? t('system.email.status.connected') : t('system.email.status.disconnected')} />
              <span className='text-muted-foreground text-sm'>{connected ? t('system.email.status.connected') : t('system.email.status.disconnected')}</span>
            </div>
          </div>
        </CardHeader>
        <CardContent className='space-y-6'>
          <div className='grid grid-cols-2 gap-4'>
            <div className='space-y-2'>
              <Label htmlFor='smtp-host'>{t('system.email.smtpHost.label')}</Label>
              <Input
                id='smtp-host'
                value={formData.smtpHost}
                onChange={(e) => handleInputChange('smtpHost', e.target.value)}
                placeholder={t('system.email.smtpHost.placeholder')}
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='smtp-port'>{t('system.email.smtpPort.label')}</Label>
              <Input
                id='smtp-port'
                type='number'
                value={formData.smtpPort}
                onChange={(e) => handleInputChange('smtpPort', parseInt(e.target.value) || 587)}
                placeholder='587'
              />
            </div>
          </div>

          <div className='grid grid-cols-2 gap-4'>
            <div className='space-y-2'>
              <Label htmlFor='smtp-username'>{t('system.email.smtpUser.label')}</Label>
              <Input
                id='smtp-username'
                value={formData.smtpUser}
                onChange={(e) => handleInputChange('smtpUser', e.target.value)}
                placeholder={t('system.email.smtpUser.placeholder')}
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='smtp-password'>{t('system.email.smtpPassword.label')}</Label>
              <Input
                id='smtp-password'
                type='password'
                value={formData.smtpPassword}
                onChange={(e) => handleInputChange('smtpPassword', e.target.value)}
                placeholder={t('system.email.smtpPassword.placeholder')}
              />
            </div>
          </div>

          <Separator />

          <div className='space-y-3'>
            <Label>{t('system.email.encryption.label')}</Label>
            <div className='text-muted-foreground text-sm'>{t('system.email.encryption.description')}</div>
            <RadioGroup
              value={formData.encryption}
              onValueChange={(value) => handleInputChange('encryption', value)}
              className='flex gap-6'
            >
              <div className='flex items-center space-x-2'>
                <RadioGroupItem value='SSL_TLS' id='encryption-ssl' />
                <Label htmlFor='encryption-ssl' className='cursor-pointer font-normal'>{t('system.email.encryption.options.sslTls')}</Label>
              </div>
              <div className='flex items-center space-x-2'>
                <RadioGroupItem value='STARTTLS' id='encryption-starttls' />
                <Label htmlFor='encryption-starttls' className='cursor-pointer font-normal'>{t('system.email.encryption.options.starttls')}</Label>
              </div>
              <div className='flex items-center space-x-2'>
                <RadioGroupItem value='NONE' id='encryption-none' />
                <Label htmlFor='encryption-none' className='cursor-pointer font-normal'>{t('system.email.encryption.options.none')}</Label>
              </div>
            </RadioGroup>
          </div>

          <Separator />

          <div className='grid grid-cols-2 gap-4'>
            <div className='space-y-2'>
              <Label htmlFor='from-name'>{t('system.email.fromName.label')}</Label>
              <Input
                id='from-name'
                value={formData.fromName}
                onChange={(e) => handleInputChange('fromName', e.target.value)}
                placeholder={t('system.email.fromName.placeholder')}
              />
            </div>
            <div className='space-y-2'>
              <Label htmlFor='from-address'>{t('system.email.fromAddress.label')}</Label>
              <Input
                id='from-address'
                type='email'
                value={formData.fromAddress}
                onChange={(e) => handleInputChange('fromAddress', e.target.value)}
                placeholder={t('system.email.fromAddress.placeholder')}
              />
            </div>
          </div>

          <Separator />

          <div className='flex items-center justify-between'>
            <Button
              type='button'
              variant='outline'
              onClick={handleTestConnection}
              disabled={testConnection.isPending}
            >
              {testConnection.isPending ? (
                <Loader2 className='mr-2 h-4 w-4 animate-spin' />
              ) : (
                <Send className='mr-2 h-4 w-4' />
              )}
              {t('system.email.testConnection')}
            </Button>

            {hasChanges && (
              <Button onClick={handleSave} disabled={updateSettings.isPending}>
                {updateSettings.isPending ? (
                  <>
                    <Loader2 className='mr-2 h-4 w-4 animate-spin' />
                    {t('system.buttons.saving')}
                  </>
                ) : (
                  t('system.buttons.save')
                )}
              </Button>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  );
}
