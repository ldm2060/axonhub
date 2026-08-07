import { HTMLAttributes, useState } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { Link } from '@tanstack/react-router';
import { Loader2, CheckCircle2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { authApi } from '@/lib/api-client';
import { cn } from '@/lib/utils';
import { passwordSchema } from '@/lib/validation';
import { Button } from '@/components/ui/button';
import { Form, FormControl, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { PasswordInput } from '@/components/password-input';

type ResetPasswordFormProps = HTMLAttributes<HTMLFormElement>;

const createFormSchema = (t: (key: string) => string) =>
  z
    .object({
      password: passwordSchema(t),
      confirmPassword: z.string(),
    })
    .refine((data) => data.password === data.confirmPassword, {
      message: t('auth.resetPassword.validation.passwordsNotMatch'),
      path: ['confirmPassword'],
    });

export function ResetPasswordForm({ token, className, ...props }: ResetPasswordFormProps & { token: string }) {
  const { t } = useTranslation();
  const [success, setSuccess] = useState(false);

  const formSchema = createFormSchema(t);
  const form = useForm<z.infer<typeof formSchema>>({
    resolver: zodResolver(formSchema),
    defaultValues: {
      password: '',
      confirmPassword: '',
    },
  });

  async function onSubmit(data: z.infer<typeof formSchema>) {
    try {
      await authApi.resetPassword(token, data.password);
      setSuccess(true);
      toast.success(t('auth.resetPassword.success'));
    } catch (error: any) {
      toast.error(error.message || t('auth.resetPassword.failed'));
    }
  }

  if (success) {
    return (
      <div className='flex flex-col items-center gap-4 text-center'>
        <div className='flex h-12 w-12 items-center justify-center rounded-full bg-emerald-100'>
          <CheckCircle2 className='h-6 w-6 text-emerald-600' />
        </div>
        <h3 className='text-lg font-medium text-slate-800'>{t('auth.resetPassword.successTitle')}</h3>
        <p className='text-sm text-slate-600'>{t('auth.resetPassword.successMessage')}</p>
        <Link to='/sign-in'>
          <Button className='bg-slate-800 text-white hover:bg-slate-700'>{t('auth.resetPassword.signIn')}</Button>
        </Link>
      </div>
    );
  }

  return (
    <Form {...form}>
      <form onSubmit={form.handleSubmit(onSubmit)} className={cn('grid gap-4', className)} {...props}>
        <FormField
          control={form.control}
          name='password'
          render={({ field }) => (
            <FormItem>
              <FormLabel className='text-sm font-medium text-slate-700'>{t('auth.resetPassword.form.password.label')}</FormLabel>
              <FormControl>
                <PasswordInput
                  placeholder={t('auth.resetPassword.form.password.placeholder')}
                  className='border-slate-300 bg-white text-slate-800 backdrop-blur-sm transition-all duration-300 placeholder:text-slate-400 focus:border-slate-500 focus:bg-white focus:ring-2 focus:ring-slate-200'
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
              <FormLabel className='text-sm font-medium text-slate-700'>{t('auth.resetPassword.form.confirmPassword.label')}</FormLabel>
              <FormControl>
                <PasswordInput
                  placeholder={t('auth.resetPassword.form.confirmPassword.placeholder')}
                  className='border-slate-300 bg-white text-slate-800 backdrop-blur-sm transition-all duration-300 placeholder:text-slate-400 focus:border-slate-500 focus:bg-white focus:ring-2 focus:ring-slate-200'
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
        >
          {form.formState.isSubmitting ? (
            <div className='flex items-center justify-center gap-2'>
              <Loader2 className='h-4 w-4 animate-spin' />
              {t('auth.resetPassword.form.submitting')}
            </div>
          ) : (
            t('auth.resetPassword.form.submit')
          )}
        </Button>
      </form>
    </Form>
  );
}
