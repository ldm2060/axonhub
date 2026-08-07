import { Link } from '@tanstack/react-router';
import { CheckCircle2, Clock, XCircle } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import AuthLayout from '@/features/auth/auth-layout';
import TwoColumnAuth from '@/features/auth/components/two-column-auth';

export default function VerifyEmail() {
  const { t } = useTranslation();

  // Read URL search params
  const searchParams = new URLSearchParams(window.location.search);
  const verified = searchParams.get('verified');
  const pending = searchParams.get('pending');

  const isVerified = verified === '1';
  const isPending = pending === '1';

  let icon: React.ReactNode;
  let title: string;
  let message: string;
  let action: React.ReactNode;

  if (isVerified && isPending) {
    // Awaiting admin approval
    icon = <Clock className='h-8 w-8 text-blue-500' />;
    title = t('auth.verifyEmail.pendingTitle');
    message = t('auth.verifyEmail.pendingMessage');
    action = (
      <Link to='/sign-in'>
        <Button className='bg-slate-800 text-white hover:bg-slate-700'>{t('auth.verifyEmail.backToSignIn')}</Button>
      </Link>
    );
  } else if (isVerified) {
    // Email verified successfully
    icon = <CheckCircle2 className='h-8 w-8 text-emerald-500' />;
    title = t('auth.verifyEmail.successTitle');
    message = t('auth.verifyEmail.successMessage');
    action = (
      <Link to='/sign-in'>
        <Button className='bg-slate-800 text-white hover:bg-slate-700'>{t('auth.verifyEmail.signIn')}</Button>
      </Link>
    );
  } else {
    // Verification failed
    icon = <XCircle className='h-8 w-8 text-red-500' />;
    title = t('auth.verifyEmail.failedTitle');
    message = t('auth.verifyEmail.failedMessage');
    action = (
      <div className='flex flex-col gap-2'>
        <Link to='/sign-up'>
          <Button variant='outline' className='border-slate-300 text-slate-700'>
            {t('auth.verifyEmail.backToSignUp')}
          </Button>
        </Link>
        <Link to='/sign-in'>
          <Button className='bg-slate-800 text-white hover:bg-slate-700'>{t('auth.verifyEmail.backToSignIn')}</Button>
        </Link>
      </div>
    );
  }

  return (
    <AuthLayout>
      <TwoColumnAuth title={title} description={message}>
        <div className='flex flex-col items-center gap-6 text-center'>
          <div className='flex h-16 w-16 items-center justify-center rounded-full bg-slate-100'>{icon}</div>
          {action}
        </div>
      </TwoColumnAuth>
    </AuthLayout>
  );
}
