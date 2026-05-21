import { HTMLAttributes, useState } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Link } from '@tanstack/react-router';
import { useTranslation } from 'react-i18next';
import { Loader2, Mail } from 'lucide-react';
import { cn } from '@/lib/utils';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { authApi } from '@/lib/api-client';
import { toast } from 'sonner';

type ForgotPasswordFormProps = HTMLAttributes<HTMLFormElement>;

const createFormSchema = (t: (key: string) => string) =>
  z.object({
    email: z.string().min(1, { message: t('auth.forgotPassword.validation.emailRequired') }).email({ message: t('auth.forgotPassword.validation.emailInvalid') }),
  });

export function ForgotPasswordForm({ className, ...props }: ForgotPasswordFormProps) {
  const { t } = useTranslation();
  const [success, setSuccess] = useState(false);
  const [submittedEmail, setSubmittedEmail] = useState('');

  const formSchema = createFormSchema(t);
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      email: '',
    },
  });

  async function onSubmit(data: z.infer<typeof formSchema>) {
    try {
      await authApi.forgotPassword(data.email);
      setSubmittedEmail(data.email);
      setSuccess(true);
    } catch (error: any) {
      // Even on error, show success message to prevent email enumeration
      setSubmittedEmail(data.email);
      setSuccess(true);
    }
  }

  if (success) {
    return (
      <div className='flex flex-col items-center gap-4 text-center'>
        <div className='flex h-12 w-12 items-center justify-center rounded-full bg-emerald-100'>
          <Mail className='h-6 w-6 text-emerald-600' />
        </div>
        <h3 className='text-lg font-medium text-slate-800'>{t('auth.forgotPassword.successTitle')}</h3>
        <p className='text-sm text-slate-600'>{t('auth.forgotPassword.successMessage')}</p>
        <Link to='/sign-in'>
          <Button className='bg-slate-800 text-white hover:bg-slate-700'>
            {t('auth.forgotPassword.backToSignIn')}
          </Button>
        </Link>
      </div>
    );
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className={cn('grid gap-4', className)} {...props}>
        <FormField
          control={form.control}
          name='email'
          render={({ field }) => (
            <FormItem>
              <FormLabel className='text-sm font-medium text-slate-700'>{t('auth.forgotPassword.form.email.label')}</FormLabel>
              <FormControl>
                <Input
                  type='email'
                  placeholder={t('auth.forgotPassword.form.email.placeholder')}
                  className='border-slate-300 !bg-white text-slate-800 transition-all duration-300 placeholder:text-slate-400 focus:border-slate-500 focus:!bg-white focus:ring-2 focus:ring-slate-200'
                  data-testid='forgot-password-email'
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
          disabled={form.formState.isSubmitting}
          data-testid='forgot-password-submit'
        >
          {form.formState.isSubmitting ? (
            <div className='flex items-center justify-center gap-2'>
              <Loader2 className='h-4 w-4 animate-spin' />
              {t('auth.forgotPassword.form.submitting')}
            </div>
          ) : (
            t('auth.forgotPassword.form.submit')
          )}
        </Button>

        <p className='text-center text-sm text-slate-600'>
          <Link to='/sign-in' className='font-medium text-slate-500 transition-colors hover:text-slate-700 hover:underline'>
            {t('auth.forgotPassword.form.backToSignIn')}
          </Link>
        </p>
      </form>
    </Form>
  );
}