'use client';

import { Suspense, lazy } from 'react';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Header } from '@/components/layout/header';
import { Main } from '@/components/layout/main';
import SystemProvider from './context/system-provider';
import { SYSTEM_PAGES, type SystemPageKey, type SystemTabKey } from './pages';

// The settings tabs pull in every settings component; keep them out of the
// entry bundle with a lazy boundary.
const SystemSettingsTabs = lazy(() => import('./components/tabs').then((m) => ({ default: m.SystemSettingsTabs })));

interface SystemContentProps {
  page: SystemPageKey;
  initialTab?: SystemTabKey;
}

function SystemContent({ page, initialTab }: SystemContentProps) {
  return (
    <div className='-mx-4 flex-1 px-4 py-1 lg:flex-row lg:space-y-0 lg:space-x-12'>
      <Suspense
        fallback={
          <div className='flex h-40 items-center justify-center'>
            <Loader2 className='text-muted-foreground h-6 w-6 animate-spin' />
          </div>
        }
      >
        <SystemSettingsTabs page={page} initialTab={initialTab} />
      </Suspense>
    </div>
  );
}

interface SystemManagementProps {
  page?: SystemPageKey;
  initialTab?: SystemTabKey;
}

export default function SystemManagement({ page = 'general', initialTab }: SystemManagementProps) {
  const { t } = useTranslation();
  const { titleKey, descriptionKey } = SYSTEM_PAGES[page];

  return (
    <SystemProvider>
      <Header fixed></Header>

      <Main>
        <div className='mb-2 flex flex-wrap items-center justify-between space-y-2'>
          <div id='system-title'>
            <h2 className='text-2xl font-bold tracking-tight'>{t(titleKey)}</h2>
            <p className='text-muted-foreground'>{t(descriptionKey)}</p>
          </div>
        </div>
        <SystemContent page={page} initialTab={initialTab} />
      </Main>
    </SystemProvider>
  );
}
