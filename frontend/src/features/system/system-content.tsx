'use client';

import { useTranslation } from 'react-i18next';
import { SystemSettingsTabs } from './components/tabs';
import { SYSTEM_PAGES, type SystemPageKey, type SystemTabKey } from './pages';

interface SystemContentProps {
  page: SystemPageKey;
  initialTab?: SystemTabKey;
}

export default function SystemContent({ page, initialTab }: SystemContentProps) {
  const { t } = useTranslation();
  const { titleKey, descriptionKey } = SYSTEM_PAGES[page];

  return (
    <>
      <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
        <div id='system-title'>
          <h2 className='text-2xl font-bold tracking-tight'>{t(titleKey)}</h2>
          <p className='text-muted-foreground'>{t(descriptionKey)}</p>
        </div>
      </div>
      <div className='-mx-4 flex-1 px-4 py-1 lg:flex-row lg:space-y-0 lg:space-x-12'>
        <SystemSettingsTabs page={page} initialTab={initialTab} />
      </div>
    </>
  );
}
