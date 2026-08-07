import { useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import AuthLayout from '../auth-layout';
import TwoColumnAuth from '../components/two-column-auth';
import AnimatedLineBackground from './components/animated-line-background';
import { UserAuthForm } from './components/user-auth-form';
import './login-styles.css';

export default function SignIn() {
  const { t } = useTranslation();

  useEffect(() => {
    const searchParams = new URLSearchParams(window.location.search);
    const verified = searchParams.get('verified');
    const pending = searchParams.get('pending');

    if (verified === '1' && pending === '1') {
      toast.info(t('auth.signIn.verificationPending'));
    } else if (verified === '1') {
      toast.success(t('auth.signIn.verificationSuccess'));
    } else if (verified === '0') {
      toast.error(t('auth.signIn.verificationFailed'));
    }

    // Clean URL params after showing toast
    if (verified) {
      window.history.replaceState({}, '', '/sign-in');
    }
  }, [t]);

  return (
    <AuthLayout>
      <div data-testid='sign-in-animation-layer'>
        <AnimatedLineBackground key='optimized-layout' />
      </div>
      <TwoColumnAuth
        title={t('auth.signIn.title')}
        description={t('auth.signIn.subtitle')}
        rightFooter={<p className='text-xs leading-relaxed text-slate-500 sm:text-sm'>{t('auth.signIn.footer.agreement')}</p>}
      >
        <UserAuthForm />
      </TwoColumnAuth>
    </AuthLayout>
  );
}
