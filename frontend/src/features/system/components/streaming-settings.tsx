'use client';

import React, { useCallback, useEffect, useState } from 'react';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { useStreamingSettings, useUpdateStreamingSettings, type UpdateStreamingSettingsInput } from '../data/system';

export function StreamingSettings() {
  const { t } = useTranslation();
  const { data: streamingSettings, isLoading } = useStreamingSettings();
  const updateStreamingSettings = useUpdateStreamingSettings();

  const [webSocketKeepalive, setWebSocketKeepalive] = useState<number>(0);
  const [httpStreamKeepalive, setHTTPStreamKeepalive] = useState<number>(0);

  useEffect(() => {
    if (streamingSettings) {
      setWebSocketKeepalive(streamingSettings.webSocketKeepaliveIntervalSeconds ?? 0);
      setHTTPStreamKeepalive(streamingSettings.httpStreamKeepaliveIntervalSeconds ?? 0);
    }
  }, [streamingSettings]);

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      const input: UpdateStreamingSettingsInput = {
        webSocketKeepaliveIntervalSeconds:
          Number.isFinite(webSocketKeepalive) && webSocketKeepalive > 0 ? Math.floor(webSocketKeepalive) : 0,
        httpStreamKeepaliveIntervalSeconds:
          Number.isFinite(httpStreamKeepalive) && httpStreamKeepalive > 0 ? Math.floor(httpStreamKeepalive) : 0,
      };
      await updateStreamingSettings.mutateAsync(input);
    },
    [updateStreamingSettings, webSocketKeepalive, httpStreamKeepalive]
  );

  if (isLoading) {
    return (
      <div className='flex items-center justify-center p-8'>
        <Loader2 className='h-8 w-8 animate-spin' />
      </div>
    );
  }

  return (
    <Card>
      <CardHeader>
        <CardTitle>{t('system.streaming.title')}</CardTitle>
        <CardDescription>{t('system.streaming.description')}</CardDescription>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className='space-y-6'>
          <div className='space-y-2'>
            <Label htmlFor='ws-keepalive'>{t('system.streaming.keepalive.label')}</Label>
            <div className='text-muted-foreground mb-2 text-sm'>{t('system.streaming.keepalive.description')}</div>
            <div className='flex items-center space-x-2'>
              <Input
                id='ws-keepalive'
                type='number'
                min='0'
                max='3600'
                value={webSocketKeepalive}
                onChange={(e) => setWebSocketKeepalive(Number(e.target.value) || 0)}
                className='w-32'
              />
              <span className='text-muted-foreground text-sm'>s</span>
            </div>
          </div>

          <div className='space-y-2'>
            <Label htmlFor='http-stream-keepalive'>{t('system.streaming.httpKeepalive.label')}</Label>
            <div className='text-muted-foreground mb-2 text-sm'>{t('system.streaming.httpKeepalive.description')}</div>
            <div className='flex items-center space-x-2'>
              <Input
                id='http-stream-keepalive'
                type='number'
                min='0'
                max='3600'
                value={httpStreamKeepalive}
                onChange={(e) => setHTTPStreamKeepalive(Number(e.target.value) || 0)}
                className='w-32'
              />
              <span className='text-muted-foreground text-sm'>s</span>
            </div>
          </div>

          <div className='flex justify-end'>
            <Button type='submit' disabled={updateStreamingSettings.isPending} className='min-w-24'>
              {updateStreamingSettings.isPending ? <Loader2 className='h-4 w-4 animate-spin' /> : t('common.buttons.save')}
            </Button>
          </div>
        </form>
      </CardContent>
    </Card>
  );
}
