import { Link, useSearch } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import AuthLayout from '../auth-layout';
import TwoColumnAuth from '../components/two-column-auth';
import AnimatedLineBackground from '../sign-in/components/animated-line-background';
import { InviteForm } from './components/invite-form';

export default function Invite() {
  const { t } = useTranslation();
  const search = useSearch({ strict: false }) as { token?: string };
  const token = search?.token ?? '';

  return (
    <AuthLayout>
      <AnimatedLineBackground key='invitation-registration' />
      <TwoColumnAuth
        title={t('users.invitation.registrationTitle')}
        description={
          <>
            {t('users.invitation.registrationDescription')}{' '}
            <Link to='/sign-in' className='font-medium text-slate-700 underline underline-offset-4 hover:text-slate-950'>
              {t('users.invitation.signIn')}
            </Link>
          </>
        }
        rightFooter={
          <p className='text-sm leading-relaxed text-slate-500'>
            {t('users.invitation.termsPrefix')}{' '}
            <a href='/terms' className='font-medium text-slate-600 underline underline-offset-4 hover:text-slate-950'>
              {t('users.invitation.terms')}
            </a>{' '}
            {t('users.invitation.termsAnd')}{' '}
            <a href='/privacy' className='font-medium text-slate-600 underline underline-offset-4 hover:text-slate-950'>
              {t('users.invitation.privacy')}
            </a>
            .
          </p>
        }
        rightMaxWidthClassName='max-w-xl'
      >
        {token ? (
          <InviteForm token={token} />
        ) : (
          <p className='border-destructive/20 bg-destructive/5 text-destructive rounded-lg border px-4 py-3 text-sm'>
            {t('users.invitation.required')}
          </p>
        )}
      </TwoColumnAuth>
    </AuthLayout>
  );
}
