'use client';

import { useCallback, useEffect, useRef, useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Tabs, TabsList, TabsTrigger, TabsContent } from '@/components/ui/tabs';
import { useHorizontalScroll } from '@/hooks/use-horizontal-scroll';
import { AboutSettings } from './about-settings';
import { BrandSettings } from './brand-settings';
import { DiagnosticsSettings } from './diagnostics-settings';
import { EmailSettingsTab } from './email-settings';
import { GeneralSettings } from './general-settings';
import { QuotaSettings } from './quota-settings';
import { RegistrationSettingsTab } from './registration-settings';
import { RetrySettings } from './retry-settings';
import { StreamingSettings } from './streaming-settings';
import { SecuritySettings } from './security-settings';
import { StorageSettings } from './storage-settings';
import { BackupSettings } from './backup-settings';
import { ProxyPresetsSettings } from './proxy-presets-settings';
import { WebhookSettings } from './webhook-settings';
import { usePermissions } from '@/hooks/usePermissions';

type SystemTabKey = 'general' | 'security' | 'brand' | 'registration' | 'email' | 'storage' | 'retry' | 'streaming' | 'webhook' | 'proxy' | 'quota' | 'backup' | 'diagnostics' | 'about';

const OWNER_ONLY_TABS: ReadonlySet<string> = new Set(['backup', 'diagnostics']);

interface SystemSettingsTabsProps {
  initialTab?: string;
}

export function SystemSettingsTabs({ initialTab }: SystemSettingsTabsProps) {
  const { t } = useTranslation();
  const { isOwner } = usePermissions();
  const [activeTab, setActiveTab] = useState<string>('general');
  const tabListRef = useRef<HTMLDivElement>(null);
  const horizontalScrollRef = useHorizontalScroll<HTMLDivElement>();
  const setTabListRef = useCallback(
    (node: HTMLDivElement | null) => {
      tabListRef.current = node;
      horizontalScrollRef(node);
    },
    [horizontalScrollRef]
  );

  useEffect(() => {
    if (!initialTab) {
      return;
    }

    if (!isOwner && OWNER_ONLY_TABS.has(initialTab)) {
      setActiveTab('general');
      return;
    }

    setActiveTab(initialTab);
  }, [initialTab, isOwner]);

  useEffect(() => {
    const tabList = tabListRef.current;
    if (!tabList) return;

    const activeTrigger = Array.from(tabList.querySelectorAll<HTMLElement>('[data-value]')).find(
      (trigger) => trigger.dataset.value === activeTab
    );
    if (!activeTrigger) return;

    const listRect = tabList.getBoundingClientRect();
    const triggerRect = activeTrigger.getBoundingClientRect();
    if (triggerRect.left < listRect.left) {
      tabList.scrollLeft -= listRect.left - triggerRect.left;
    } else if (triggerRect.right > listRect.right) {
      tabList.scrollLeft += triggerRect.right - listRect.right;
    }
  }, [activeTab]);

  return (
    <Tabs
      value={activeTab}
      onValueChange={(value) => {
        if (!isOwner && OWNER_ONLY_TABS.has(value)) {
          setActiveTab('general');
          return;
        }
        setActiveTab(value);
      }}
      className='w-full'
    >
      <TabsList
        ref={setTabListRef}
        className='shadow-soft border-border bg-background flex w-full justify-start overflow-x-auto rounded-2xl border sm:overflow-x-visible [&_[data-slot=tabs-trigger]]:flex-none [&_[data-slot=tabs-trigger]]:shrink-0 sm:[&_[data-slot=tabs-trigger]]:flex-1 sm:[&_[data-slot=tabs-trigger]]:shrink'
      >
        <TabsTrigger value='general' data-value='general'>
          {t('system.tabs.general')}
        </TabsTrigger>
        <TabsTrigger value='security' data-value='security'>
          {t('system.tabs.security')}
        </TabsTrigger>
        <TabsTrigger value='brand' data-value='brand'>
          {t('system.tabs.brand')}
        </TabsTrigger>
        {isOwner && (
          <TabsTrigger value='registration' data-value='registration'>
            {t('system.tabs.registration')}
          </TabsTrigger>
        )}
        {isOwner && (
          <TabsTrigger value='email' data-value='email'>
            {t('system.tabs.email')}
          </TabsTrigger>
        )}
        <TabsTrigger value='retry' data-value='retry'>
          {t('system.tabs.retry')}
        </TabsTrigger>
        <TabsTrigger value='streaming' data-value='streaming'>
          {t('system.tabs.streaming')}
        </TabsTrigger>
        <TabsTrigger value='webhook' data-value='webhook'>
          {t('system.tabs.webhook')}
        </TabsTrigger>
        <TabsTrigger value='storage' data-value='storage'>
          {t('system.tabs.storage')}
        </TabsTrigger>
        <TabsTrigger value='proxy' data-value='proxy'>
          {t('system.tabs.proxy')}
        </TabsTrigger>
        <TabsTrigger value='quota' data-value='quota'>
          {t('system.tabs.quota')}
        </TabsTrigger>
        {isOwner && (
          <TabsTrigger value='diagnostics' data-value='diagnostics'>
            {t('system.tabs.diagnostics')}
          </TabsTrigger>
        )}
        {isOwner && (
          <TabsTrigger value='backup' data-value='backup'>
            {t('system.tabs.backup')}
          </TabsTrigger>
        )}
        <TabsTrigger value='about' data-value='about'>
          {t('system.tabs.about')}
        </TabsTrigger>
      </TabsList>
      <div className='shadow-soft border-border bg-card mt-6 rounded-2xl border p-4 sm:p-6'>
        <TabsContent value='general' className='mt-0 p-0'>
          <GeneralSettings />
        </TabsContent>
        <TabsContent value='security' className='mt-0 p-0'>
          <SecuritySettings />
        </TabsContent>
        <TabsContent value='brand' className='mt-0 p-0'>
          <BrandSettings />
        </TabsContent>
        {isOwner && (
          <TabsContent value='registration' className='mt-0 p-0'>
            <RegistrationSettingsTab />
          </TabsContent>
        )}
        {isOwner && (
          <TabsContent value='email' className='mt-0 p-0'>
            <EmailSettingsTab />
          </TabsContent>
        )}
        <TabsContent value='storage' className='mt-0 p-0'>
          <StorageSettings />
        </TabsContent>
        <TabsContent value='retry' className='mt-0 p-0'>
          <RetrySettings />
        </TabsContent>
        <TabsContent value='streaming' className='mt-0 p-0'>
          <StreamingSettings />
        </TabsContent>
        <TabsContent value='webhook' className='mt-0 p-0'>
          <WebhookSettings />
        </TabsContent>
        <TabsContent value='proxy' className='mt-0 p-0'>
          <ProxyPresetsSettings />
        </TabsContent>
        <TabsContent value='quota' className='mt-0 p-0'>
          <QuotaSettings />
        </TabsContent>
        {isOwner && (
          <TabsContent value='diagnostics' className='mt-0 p-0'>
            <DiagnosticsSettings />
          </TabsContent>
        )}
        {isOwner && (
          <TabsContent value='backup' className='mt-0 p-0'>
            <BackupSettings />
          </TabsContent>
        )}
        <TabsContent value='about' className='mt-0 p-0'>
          <AboutSettings />
        </TabsContent>
      </div>
    </Tabs>
  );
}
