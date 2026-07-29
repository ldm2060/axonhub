'use client';

import { useTranslation } from 'react-i18next';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import { RuntimeOverview } from './components/runtime-overview';

export default function RuntimeManagement() {
  const { t } = useTranslation();

  return (
    <>
      <Header fixed></Header>

      <Main>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div id='runtime-title'>
            <h2 className='text-2xl font-bold tracking-tight'>{t('runtime.title')}</h2>
            <p className='text-muted-foreground'>{t('runtime.description')}</p>
          </div>
        </div>
        <div className='-mx-4 flex-1 overflow-auto px-4 py-1'>
          <RuntimeOverview />
        </div>
      </Main>
    </>
  );
}
