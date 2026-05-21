import { HTMLAttributes, useState, useCallback } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Link } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { Mail, Loader2 } from 'lucide-react';
import { cn } from '@/lib/utils';
import { passwordSchema } from '@/lib/validation';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { PasswordInput } from '@/components/password-input';
import { useSignUp } from '@/features/auth/data/auth';
import { authApi } from '@/lib/api-client';
import { toast } from 'sonner';

type SignUpFormProps = HTMLAttributes<HTMLFormElement>;

const createFormSchema = (t: (key: string) => string) =>
  z
    .object({
      firstName: z.string().min(1, { message: t('auth.signUp.validation.firstNameRequired') }),
      lastName: z.string().min(1, { message: t('auth.signUp.validation.lastNameRequired') }),
      email: z.string().min(1, { message: t('auth.signUp.validation.emailRequired') }).email({ message: t('auth.signUp.validation.emailInvalid') }),
      password: passwordSchema(t),
      confirmPassword: z.string(),
    })
    .refine((data) => data.password === data.confirmPassword, {
      message: t('auth.signUp.validation.passwordsNotMatch'),
      path: ['confirmPassword'],
    });

export function SignUpForm({ className, ...props }: SignUpFormProps) {
  const { t } = useTranslation();
  const signUpMutation = useSignUp();
  const [successState, setSuccessState] = useState<{ email: string; pending: boolean } | null>(null);
  const [resendLoading, setResendLoading] = useState(false);

  const formSchema = createFormSchema(t);
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      firstName: '',
      lastName: '',
      email: '',
      password: '',
      confirmPassword: '',
    },
  });

  const onSubmit = useCallback((data: z.infer<typeof formSchema>) => {
    const email = data.email;
    signUpMutation.mutate(
      {
        email: data.email,
        password: data.password,
        first_name: data.firstName,
        last_name: data.lastName,
      },
      {
        onSuccess: (responseData: any) => {
          const pending = responseData?.pending === true;
          setSuccessState({ email, pending });
        },
      }
    );
  }, [signUpMutation]);

  const handleResendVerification = useCallback(async () => {
    if (!successState) return;
    setResendLoading(true);
    try {
      await authApi.resendVerification(successState.email);
      toast.success(t('auth.signUp.resendSuccess'));
    } catch (error: any) {
      toast.error(error.message || t('auth.signUp.resendFailed'));
    } finally {
      setResendLoading(false);
    }
  }, [successState, t]);

  if (successState) {
    return (
      <div className='flex flex-col items-center gap-4 text-center'>
        <div className='flex h-12 w-12 items-center justify-center rounded-full bg-emerald-100'>
          <Mail className='h-6 w-6 text-emerald-600' />
        </div>
        <h3 className='text-lg font-medium text-slate-800'>
          {successState.pending ? t('auth.signUp.successPendingTitle') : t('auth.signUp.successTitle')}
        </h3>
        <p className='text-sm text-slate-600'>
          {successState.pending
            ? t('auth.signUp.successPendingMessage')
            : t('auth.signUp.successMessage', { email: successState.email })}
        </p>
        {!successState.pending && (
          <p className='text-sm text-slate-600'>
            {t('auth.signUp.successEmailSent', { email: successState.email })}
          </p>
        )}
        <div className='flex flex-col gap-2'>
          {!successState.pending && (
            <Button
              variant='outline'
              onClick={handleResendVerification}
              disabled={resendLoading}
              className='border-slate-300 text-slate-700'
            >
              {resendLoading ? <Loader2 className='mr-2 h-4 w-4 animate-spin' /> : null}
              {t('auth.signUp.resendVerification')}
            </Button>
          )}
          <Link to='/sign-in'>
            <Button className='w-full bg-slate-800 text-white hover:bg-slate-700'>
              {t('auth.signUp.backToSignIn')}
            </Button>
          </Link>
        </div>
      </div>
    );
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className={cn('grid gap-4', className)} {...props}>
        <div className='grid grid-cols-2 gap-3'>
          <FormField
            control={form.control}
            name='firstName'
            render={({ field }) => (
              <FormItem>
                <FormLabel className='text-sm font-medium text-slate-700'>{t('auth.signUp.form.firstName.label')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t('auth.signUp.form.firstName.placeholder')}
                    className='border-slate-300 !bg-white text-slate-800 transition-all duration-300 placeholder:text-slate-400 focus:border-slate-500 focus:!bg-white focus:ring-2 focus:ring-slate-200'
                    data-testid='sign-up-first-name'
                    {...field}
                  />
                </FormControl>
                <FormMessage className='text-red-600' />
              </FormItem>
            )}
          />
          <FormField
            control={form.control}
            name='lastName'
            render={({ field }) => (
              <FormItem>
                <FormLabel className='text-sm font-medium text-slate-700'>{t('auth.signUp.form.lastName.label')}</FormLabel>
                <FormControl>
                  <Input
                    placeholder={t('auth.signUp.form.lastName.placeholder')}
                    className='border-slate-300 !bg-white text-slate-800 transition-all duration-300 placeholder:text-slate-400 focus:border-slate-500 focus:!bg-white focus:ring-2 focus:ring-slate-200'
                    data-testid='sign-up-last-name'
                    {...field}
                  />
                </FormControl>
                <FormMessage className='text-red-600' />
              </FormItem>
            )}
          />
        </div>

        <FormField
          control={form.control}
          name='email'
          render={({ field }) => (
            <FormItem>
              <FormLabel className='text-sm font-medium text-slate-700'>{t('auth.signUp.form.email.label')}</FormLabel>
              <FormControl>
                <Input
                  type='email'
                  placeholder={t('auth.signUp.form.email.placeholder')}
                  className='border-slate-300 !bg-white text-slate-800 transition-all duration-300 placeholder:text-slate-400 focus:border-slate-500 focus:!bg-white focus:ring-2 focus:ring-slate-200'
                  data-testid='sign-up-email'
                  {...field}
                />
              </FormControl>
              <FormMessage className='text-red-600' />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='password'
          render={({ field }) => (
            <FormItem>
              <FormLabel className='text-sm font-medium text-slate-700'>{t('auth.signUp.form.password.label')}</FormLabel>
              <FormControl>
                <PasswordInput
                  placeholder={t('auth.signUp.form.password.placeholder')}
                  className='border-slate-300 bg-white text-slate-800 backdrop-blur-sm transition-all duration-300 placeholder:text-slate-400 focus:border-slate-500 focus:bg-white focus:ring-2 focus:ring-slate-200'
                  data-testid='sign-up-password'
                  {...field}
                />
              </FormControl>
              <FormMessage className='text-red-600' />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='confirmPassword'
          render={({ field }) => (
            <FormItem>
              <FormLabel className='text-sm font-medium text-slate-700'>{t('auth.signUp.form.confirmPassword.label')}</FormLabel>
              <FormControl>
                <PasswordInput
                  placeholder={t('auth.signUp.form.confirmPassword.placeholder')}
                  className='border-slate-300 bg-white text-slate-800 backdrop-blur-sm transition-all duration-300 placeholder:text-slate-400 focus:border-slate-500 focus:bg-white focus:ring-2 focus:ring-slate-200'
                  data-testid='sign-up-confirm-password'
                  {...field}
                />
              </FormControl>
              <FormMessage className='text-red-600' />
            </FormItem>
          )}
        />

        <Button
          type='submit'
          className='mt-2 w-full rounded-lg bg-slate-800 px-6 py-3 font-medium text-white shadow-lg transition-all duration-300 hover:bg-slate-700 hover:shadow-xl focus:ring-2 focus:ring-slate-500 focus:ring-offset-2 disabled:opacity-50'
          disabled={signUpMutation.isPending}
          data-testid='sign-up-submit'
        >
          {signUpMutation.isPending ? (
            <div className='flex items-center justify-center gap-2'>
              <div className='h-4 w-4 animate-spin rounded-full border-2 border-white/30 border-t-white'></div>
              {t('auth.signUp.form.signingUp')}
            </div>
          ) : (
            t('auth.signUp.form.signUpButton')
          )}
        </Button>

        <p className='text-center text-sm text-slate-600'>
          {t('auth.signUp.form.hasAccount')}{' '}
          <Link to='/sign-in' className='font-medium text-slate-500 transition-colors hover:text-slate-700 hover:underline'>
            {t('auth.signUp.form.signInLink')}
          </Link>
        </p>
      </form>
    </Form>
  );
}
