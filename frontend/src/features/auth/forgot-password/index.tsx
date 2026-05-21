import { Link } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import AuthLayout from '../auth-layout';
import TwoColumnAuth from '../components/two-column-auth';
import { ForgotPasswordForm } from './components/forgot-password-form';

export default function ForgotPassword() {
  const { t } = useTranslation();

  return (
    <AuthLayout>
      <TwoColumnAuth
        title={t('auth.forgotPassword.title')}
        description={t('auth.forgotPassword.description')}
      >
        <ForgotPasswordForm />
        <p className='text-muted-foreground mt-4 text-center text-sm'>
          {t('auth.forgotPassword.noAccount')}{' '}
          <Link to='/sign-up' className='hover:text-primary underline underline-offset-4'>
            {t('auth.forgotPassword.signUp')}
          </Link>
        </p>
      </TwoColumnAuth>
    </AuthLayout>
  );
}
