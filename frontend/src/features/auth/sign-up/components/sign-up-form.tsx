import { HTMLAttributes, useState, useCallback, useEffect, useRef } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Link } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { Loader2, CheckCircle, Clock } from 'lucide-react';
import { cn } from '@/lib/utils';
import { passwordSchema } from '@/lib/validation';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { PasswordInput } from '@/components/password-input';
import { useSignUp, useSendVerificationCode } from '@/features/auth/data/auth';
import { toast } from 'sonner';

type SignUpFormProps = HTMLAttributes<HTMLFormElement>;

const CODE_COUNTDOWN_SECONDS = 60;

const createFormSchema = (t: (key: string) => string) =>
  z
    .object({
      firstName: z.string().min(1, { message: t('auth.signUp.validation.firstNameRequired') }),
      lastName: z.string().min(1, { message: t('auth.signUp.validation.lastNameRequired') }),
      email: z.string().min(1, { message: t('auth.signUp.validation.emailRequired') }).email({ message: t('auth.signUp.validation.emailInvalid') }),
      verificationCode: z.string().min(1, { message: t('auth.signUp.validation.verificationCodeRequired') }).regex(/^\d{6}$/, { message: t('auth.signUp.validation.verificationCodeInvalid') }),
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
  const sendCodeMutation = useSendVerificationCode();
  const [successState, setSuccessState] = useState<{ pending: boolean } | null>(null);
  const [countdown, setCountdown] = useState(0);
  const timerRef = useRef<ReturnType<typeof setInterval> | null>(null);

  const formSchema = createFormSchema(t);
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      firstName: '',
      lastName: '',
      email: '',
      verificationCode: '',
      password: '',
      confirmPassword: '',
    },
  });

  const emailValue = form.watch('email');
  const emailValid = !form.formState.errors.email && emailValue && /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(emailValue);

  useEffect(() => {
    return () => {
      if (timerRef.current) clearInterval(timerRef.current);
    };
  }, []);

  const startCountdown = useCallback(() => {
    setCountdown(CODE_COUNTDOWN_SECONDS);
    if (timerRef.current) clearInterval(timerRef.current);
    timerRef.current = setInterval(() => {
      setCountdown((prev) => {
        if (prev <= 1) {
          if (timerRef.current) clearInterval(timerRef.current);
          return 0;
        }
        return prev - 1;
      });
    }, 1000);
  }, []);

  const handleSendCode = useCallback(async () => {
    if (!emailValid || countdown > 0) return;
    try {
      await sendCodeMutation.mutateAsync(emailValue);
      toast.success(t('auth.signUp.form.verificationCode.sentSuccess'));
      startCountdown();
    } catch {
      // Error handled by mutation onError
    }
  }, [emailValid, emailValue, countdown, sendCodeMutation, t, startCountdown]);

  const onSubmit = useCallback((data: z.infer<typeof formSchema>) => {
    signUpMutation.mutate(
      {
        email: data.email,
        password: data.password,
        first_name: data.firstName,
        last_name: data.lastName,
        verification_code: data.verificationCode,
      },
      {
        onSuccess: (responseData: any) => {
          const pending = responseData?.pending === true;
          setSuccessState({ pending });
        },
      }
    );
  }, [signUpMutation]);

  if (successState) {
    return (
      <div className='flex flex-col items-center gap-4 text-center'>
        {successState.pending ? (
          <div className='flex h-12 w-12 items-center justify-center rounded-full bg-amber-100'>
            <Clock className='h-6 w-6 text-amber-600' />
          </div>
        ) : (
          <div className='flex h-12 w-12 items-center justify-center rounded-full bg-emerald-100'>
            <CheckCircle className='h-6 w-6 text-emerald-600' />
          </div>
        )}
        <h3 className='text-lg font-medium text-slate-800'>
          {successState.pending ? t('auth.signUp.successPendingTitle') : t('auth.signUp.successActivatedTitle')}
        </h3>
        <p className='text-sm text-slate-600'>
          {successState.pending ? t('auth.signUp.successPendingMessage') : t('auth.signUp.successActivatedMessage')}
        </p>
        <Link to='/sign-in'>
          <Button className='w-full bg-slate-800 text-white hover:bg-slate-700'>
            {t('auth.signUp.backToSignIn')}
          </Button>
        </Link>
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
              <div className='flex gap-2'>
                <FormControl>
                  <Input
                    type='email'
                    placeholder={t('auth.signUp.form.email.placeholder')}
                    className='border-slate-300 !bg-white text-slate-800 transition-all duration-300 placeholder:text-slate-400 focus:border-slate-500 focus:!bg-white focus:ring-2 focus:ring-slate-200'
                    data-testid='sign-up-email'
                    {...field}
                  />
                </FormControl>
                <Button
                  type='button'
                  variant='outline'
                  onClick={handleSendCode}
                  disabled={!emailValid || countdown > 0 || sendCodeMutation.isPending}
                  className='shrink-0 border-slate-300 text-slate-700 whitespace-nowrap'
                  data-testid='sign-up-send-code'
                >
                  {sendCodeMutation.isPending ? (
                    <Loader2 className='h-4 w-4 animate-spin' />
                  ) : countdown > 0 ? (
                    t('auth.signUp.form.verificationCode.resendIn', { seconds: countdown })
                  ) : (
                    t('auth.signUp.form.verificationCode.send')
                  )}
                </Button>
              </div>
              <FormMessage className='text-red-600' />
            </FormItem>
          )}
        />

        <FormField
          control={form.control}
          name='verificationCode'
          render={({ field }) => (
            <FormItem>
              <FormLabel className='text-sm font-medium text-slate-700'>{t('auth.signUp.form.verificationCode.label')}</FormLabel>
              <FormControl>
                <Input
                  placeholder={t('auth.signUp.form.verificationCode.placeholder')}
                  maxLength={6}
                  className='border-slate-300 !bg-white text-slate-800 transition-all duration-300 placeholder:text-slate-400 focus:border-slate-500 focus:!bg-white focus:ring-2 focus:ring-slate-200'
                  data-testid='sign-up-verification-code'
                  {...field}
                  onChange={(e) => {
                    const value = e.target.value.replace(/\D/g, '').slice(0, 6);
                    field.onChange(value);
                  }}
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
