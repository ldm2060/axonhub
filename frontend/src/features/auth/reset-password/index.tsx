import { useTranslation } from 'react-i18next';
import { XCircle } from 'lucide-react';
import AuthLayout from '../auth-layout';
import TwoColumnAuth from '../components/two-column-auth';
import { ResetPasswordForm } from './components/reset-password-form';

export default function ResetPassword() {
  const { t } = useTranslation();

  // Read token from URL params
  const searchParams = new URLSearchParams(window.location.search);
  const token = searchParams.get('token');

  if (!token) {
    return (
      <AuthLayout>
        <TwoColumnAuth
          title={t('auth.resetPassword.invalidTitle')}
          description={t('auth.resetPassword.invalidMessage')}
        >
          <div className='flex flex-col items-center gap-4 text-center'>
            <div className='flex h-16 w-16 items-center justify-center rounded-full bg-red-100'>
              <XCircle className='h-8 w-8 text-red-500' />
            </div>
          </div>
        </TwoColumnAuth>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout>
      <TwoColumnAuth
        title={t('auth.resetPassword.title')}
        description={t('auth.resetPassword.description')}
      >
        <ResetPasswordForm token={token} />
      </TwoColumnAuth>
    </AuthLayout>
  );
}