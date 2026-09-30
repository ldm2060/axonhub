'use client';

import { useEffect, useState } from 'react';
import { z } from 'zod';
import { useForm } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import { useTranslation } from 'react-i18next';
import { Button } from '@/components/ui/button';
import { Dialog, DialogContent, DialogDescription, DialogFooter, DialogHeader, DialogTitle } from '@/components/ui/dialog';
import { Form, FormControl, FormDescription, FormField, FormItem, FormLabel, FormMessage } from '@/components/ui/form';
import { Input } from '@/components/ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Textarea } from '@/components/ui/textarea';
import { useOAuthApplicationsContext } from '../context/oauth-applications-context';
import { useCreateOAuthApplication } from '../data/oauth-applications';
import { ClientSecretReveal } from './client-secret-reveal';

const createFormSchema = (t: (key: string) => string) =>
  z.object({
    name: z.string().min(1, { message: t('oauthApplications.validation.nameRequired') }),
    description: z.string().optional(),
    clientType: z.enum(['confidential', 'public']),
    redirectUrisText: z
      .string()
      .min(1, { message: t('oauthApplications.validation.redirectUrisRequired') })
      .refine(
        (value) =>
          value
            .split('\n')
            .map((line) => line.trim())
            .filter(Boolean)
            .every((line) => {
              try {
                const parsed = new URL(line);
                return !!parsed.protocol && !!parsed.host;
              } catch {
                return false;
              }
            }),
        { message: t('oauthApplications.validation.invalidRedirectUri') }
      ),
  });

type CreateFormValues = z.infer<ReturnType<typeof createFormSchema>>;

export function CreateOAuthApplicationDialog() {
  const { t } = useTranslation();
  const { isCreateDialogOpen, setIsCreateDialogOpen } = useOAuthApplicationsContext();
  const createMutation = useCreateOAuthApplication();
  const [createdSecret, setCreatedSecret] = useState<string | null>(null);

  const form = useForm<CreateFormValues>({
    resolver: zodResolver(createFormSchema(t)),
    defaultValues: {
      name: '',
      description: '',
      clientType: 'confidential',
      redirectUrisText: '',
    },
  });

  useEffect(() => {
    if (isCreateDialogOpen) {
      form.reset({
        name: '',
        description: '',
        clientType: 'confidential',
        redirectUrisText: '',
      });
      setCreatedSecret(null);
    }
  }, [isCreateDialogOpen, form]);

  const handleClose = () => {
    setIsCreateDialogOpen(false);
    setCreatedSecret(null);
  };

  const onSubmit = async (values: CreateFormValues) => {
    const redirectUris = values.redirectUrisText
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean);

    try {
      const result = await createMutation.mutateAsync({
        name: values.name,
        description: values.description,
        clientType: values.clientType,
        redirectUris,
      });
      setCreatedSecret(result.clientSecret);
    } catch {
      // Error is handled by the mutation
    }
  };

  return (
    <Dialog open={isCreateDialogOpen} onOpenChange={handleClose}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader className='text-left'>
          <DialogTitle>{t('oauthApplications.dialogs.create.title')}</DialogTitle>
          <DialogDescription>{t('oauthApplications.dialogs.create.description')}</DialogDescription>
        </DialogHeader>

        {createdSecret ? (
          <div className='space-y-4'>
            <ClientSecretReveal clientSecret={createdSecret} />
            <DialogFooter>
              <Button onClick={handleClose}>{t('common.buttons.close')}</Button>
            </DialogFooter>
          </div>
        ) : (
          <Form {...form}>
            <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
              <FormField
                control={form.control}
                name='name'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('oauthApplications.fields.name')}</FormLabel>
                    <FormControl>
                      <Input placeholder={t('oauthApplications.fields.namePlaceholder')} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='description'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('oauthApplications.fields.description')}</FormLabel>
                    <FormControl>
                      <Input placeholder={t('oauthApplications.fields.descriptionPlaceholder')} {...field} />
                    </FormControl>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='clientType'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('oauthApplications.fields.clientType')}</FormLabel>
                    <Select onValueChange={field.onChange} value={field.value}>
                      <FormControl>
                        <SelectTrigger>
                          <SelectValue />
                        </SelectTrigger>
                      </FormControl>
                      <SelectContent>
                        <SelectItem value='confidential'>{t('oauthApplications.clientTypes.confidential')}</SelectItem>
                        <SelectItem value='public'>{t('oauthApplications.clientTypes.public')}</SelectItem>
                      </SelectContent>
                    </Select>
                    <FormDescription>{t('oauthApplications.fields.clientTypeHint')}</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <FormField
                control={form.control}
                name='redirectUrisText'
                render={({ field }) => (
                  <FormItem>
                    <FormLabel>{t('oauthApplications.fields.redirectUris')}</FormLabel>
                    <FormControl>
                      <Textarea rows={4} placeholder={'https://app.example.com/oauth/callback'} {...field} />
                    </FormControl>
                    <FormDescription>{t('oauthApplications.fields.redirectUrisHint')}</FormDescription>
                    <FormMessage />
                  </FormItem>
                )}
              />

              <DialogFooter>
                <Button type='button' variant='outline' onClick={handleClose} disabled={createMutation.isPending}>
                  {t('common.buttons.cancel')}
                </Button>
                <Button type='submit' disabled={createMutation.isPending}>
                  {createMutation.isPending ? t('common.buttons.creating') : t('common.buttons.create')}
                </Button>
              </DialogFooter>
            </form>
          </Form>
        )}
      </DialogContent>
    </Dialog>
  );
}
