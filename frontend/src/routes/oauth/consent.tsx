import { useEffect } from 'react';
import { createFileRoute } from '@tanstack/react-router';
import { Loader2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { useApproveOAuthAuthorization, useDenyOAuthAuthorization, useOAuthAuthorizationRequest } from '@/features/oauth-applications/data/oauth-applications';
import { useAuthStore } from '@/stores/authStore';

export const Route = createFileRoute('/oauth/consent')({
  validateSearch: (search: Record<string, unknown>) => {
    return {
      request_id: typeof search.request_id === 'string' ? search.request_id : '',
    };
  },
  component: OAuthConsentPage,
});

function OAuthConsentPage() {
  const { t } = useTranslation();
  const { request_id } = Route.useSearch();
  const { accessToken } = useAuthStore((state) => state.auth);
  const requestQuery = useOAuthAuthorizationRequest(accessToken ? request_id : '');
  const approveMutation = useApproveOAuthAuthorization();
  const denyMutation = useDenyOAuthAuthorization();

  const currentUrl = () => window.location.pathname + window.location.search;

  useEffect(() => {
    if (!accessToken) {
      window.location.href = `/sign-in?redirect=${encodeURIComponent(currentUrl())}`;
    }
  }, [accessToken]);

  const handleDecision = async (approve: boolean) => {
    try {
      const redirectUrl = approve
        ? await approveMutation.mutateAsync(request_id)
        : await denyMutation.mutateAsync(request_id);
      window.location.href = redirectUrl;
    } catch {
      // Error is surfaced by the query below; keep the page in place.
    }
  };

  const pending = approveMutation.isPending || denyMutation.isPending;
  const request = requestQuery.data;

  return (
    <div className='flex min-h-screen w-full items-center justify-center bg-slate-50 p-4'>
      <div className='w-full max-w-md space-y-6 rounded-xl bg-white p-8 shadow-lg'>
        {requestQuery.isLoading ? (
          <div className='flex flex-col items-center space-y-4 text-center'>
            <Loader2 className='h-8 w-8 animate-spin text-slate-800' />
            <p className='text-sm text-slate-500'>{t('oauthApplications.consent.loading')}</p>
          </div>
        ) : requestQuery.isError || !request ? (
          <div className='space-y-2 text-center'>
            <h2 className='text-xl font-semibold text-slate-900'>{t('oauthApplications.consent.invalidTitle')}</h2>
            <p className='text-sm text-slate-500'>{t('oauthApplications.consent.invalidDescription')}</p>
          </div>
        ) : (
          <>
            <div className='space-y-2 text-center'>
              <h2 className='text-xl font-semibold text-slate-900'>{t('oauthApplications.consent.title')}</h2>
              <p className='text-sm text-slate-500'>
                {t('oauthApplications.consent.description', { client: request.clientName, host: request.redirectUriHost })}
              </p>
            </div>

            <div className='space-y-3 rounded-lg border border-slate-200 p-4'>
              <div className='flex items-center gap-3'>
                <div className='flex h-10 w-10 items-center justify-center rounded-full bg-slate-100 text-base font-semibold text-slate-700'>
                  {request.clientName.slice(0, 1).toUpperCase()}
                </div>
                <div className='flex flex-col'>
                  <span className='font-semibold text-slate-900'>{request.clientName}</span>
                  {request.clientDescription ? <span className='text-xs text-slate-500'>{request.clientDescription}</span> : null}
                </div>
              </div>

              <div className='space-y-1'>
                <p className='text-sm font-medium text-slate-900'>{t('oauthApplications.consent.requestedAccess')}</p>
                <ul className='list-inside list-disc text-sm text-slate-600'>
                  {request.scopes.map((scope) => (
                    <li key={scope}>{t(`oauthApplications.consent.scopes.${scope}`, scope)}</li>
                  ))}
                </ul>
              </div>

              <p className='text-xs text-slate-500'>{t('oauthApplications.consent.signedInAs', { email: request.userEmail })}</p>
            </div>

            <div className='flex gap-3'>
              <button
                type='button'
                disabled={pending}
                onClick={() => handleDecision(false)}
                className='flex-1 rounded-md border border-slate-300 px-4 py-2 text-sm font-medium text-slate-700 transition-colors hover:bg-slate-50 disabled:opacity-50'
              >
                {t('oauthApplications.consent.deny')}
              </button>
              <button
                type='button'
                disabled={pending}
                onClick={() => handleDecision(true)}
                className='flex-1 rounded-md bg-slate-900 px-4 py-2 text-sm font-medium text-white transition-colors hover:bg-slate-800 disabled:opacity-50'
              >
                {pending ? t('common.buttons.processing') : t('oauthApplications.consent.approve')}
              </button>
            </div>
          </>
        )}
      </div>
    </div>
  );
}
