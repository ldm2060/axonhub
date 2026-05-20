'use client';

import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { Tabs, TabsContent, TabsList, TabsTrigger } from '@/components/ui/tabs';
import { ScrollArea } from '@/components/ui/scroll-area';
import { SharedChannelsTable } from './components/shared-channels-table';
import { SharedModelsTable } from './components/shared-models-table';

export default function SharedWithMePage() {
  const { t } = useTranslation();
  const [activeTab, setActiveTab] = useState('channels');

  return (
    <>
      <Header fixed>
        <h2 className='text-xl font-bold tracking-tight'>{t('shared.title')}</h2>
      </Header>
      <Main fixed>
        <div className='flex flex-1 flex-col overflow-hidden'>
          <Tabs value={activeTab} onValueChange={setActiveTab}>
            <TabsList>
              <TabsTrigger value='channels'>{t('shared.tabs.channels')}</TabsTrigger>
              <TabsTrigger value='models'>{t('shared.tabs.models')}</TabsTrigger>
            </TabsList>
            <TabsContent value='channels' className='mt-4'>
              <ScrollArea className='flex-1'>
                <SharedChannelsTable />
              </ScrollArea>
            </TabsContent>
            <TabsContent value='models' className='mt-4'>
              <ScrollArea className='flex-1'>
                <SharedModelsTable />
              </ScrollArea>
            </TabsContent>
          </Tabs>
        </div>
      </Main>
    </>
  );
}
