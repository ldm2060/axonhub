import { Link } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import AuthLayout from '../auth-layout';
import TwoColumnAuth from '../components/two-column-auth';
import { useSignUpAllowed } from '../data/auth';
import AnimatedLineBackground from '../sign-in/components/animated-line-background';
import { SignUpForm } from './components/sign-up-form';

export default function SignUp() {
  const { t } = useTranslation();
  const { data: signUpAllowed, isLoading } = useSignUpAllowed();

  if (isLoading) {
    return (
      <AuthLayout>
        <AnimatedLineBackground key='sign-up-loading' />
        <TwoColumnAuth title={t('auth.signUp.title')} description={t('auth.signUp.subtitle')}>
          <div className='flex items-center justify-center py-8'>
            <div className='h-6 w-6 animate-spin rounded-full border-2 border-slate-300 border-t-slate-800'></div>
          </div>
        </TwoColumnAuth>
      </AuthLayout>
    );
  }

  if (signUpAllowed === false) {
    return (
      <AuthLayout>
        <AnimatedLineBackground key='sign-up-not-allowed' />
        <TwoColumnAuth title={t('auth.signUp.title')} description={t('auth.signUp.notAllowed')}>
          <div className='text-center'>
            <Link
              to='/sign-in'
              className='inline-block rounded-lg bg-slate-800 px-6 py-3 font-medium text-white shadow-lg transition-all duration-300 hover:bg-slate-700 hover:shadow-xl'
            >
              {t('auth.signUp.backToSignIn')}
            </Link>
          </div>
        </TwoColumnAuth>
      </AuthLayout>
    );
  }

  return (
    <AuthLayout>
      <AnimatedLineBackground key='sign-up-form' />
      <TwoColumnAuth
        title={t('auth.signUp.title')}
        description={t('auth.signUp.subtitle')}
        rightMaxWidthClassName='max-w-md'
        rightFooter={<p className='text-xs leading-relaxed text-slate-500 sm:text-sm'>{t('auth.signUp.footer.agreement')}</p>}
      >
        <SignUpForm />
      </TwoColumnAuth>
    </AuthLayout>
  );
}
