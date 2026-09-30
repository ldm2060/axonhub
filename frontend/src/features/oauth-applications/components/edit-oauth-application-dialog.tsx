'use client';

import { useEffect } from 'react';
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
import { useUpdateOAuthApplication } from '../data/oauth-applications';

const editFormSchema = (t: (key: string) => string) =>
  z.object({
    name: z.string().min(1, { message: t('oauthApplications.validation.nameRequired') }),
    description: z.string().optional(),
    status: z.enum(['enabled', 'disabled']),
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

type EditFormValues = z.infer<ReturnType<typeof editFormSchema>>;

export function EditOAuthApplicationDialog() {
  const { t } = useTranslation();
  const { isEditDialogOpen, setIsEditDialogOpen, editingApplication, setEditingApplication } = useOAuthApplicationsContext();
  const updateMutation = useUpdateOAuthApplication();

  const form = useForm<EditFormValues>({
    resolver: zodResolver(editFormSchema(t)),
    defaultValues: {
      name: '',
      description: '',
      status: 'enabled',
      redirectUrisText: '',
    },
  });

  useEffect(() => {
    if (isEditDialogOpen && editingApplication) {
      form.reset({
        name: editingApplication.name,
        description: editingApplication.description ?? '',
        status: editingApplication.status,
        redirectUrisText: editingApplication.redirectUris.join('\n'),
      });
    }
  }, [isEditDialogOpen, editingApplication, form]);

  const handleClose = () => {
    setIsEditDialogOpen(false);
    setEditingApplication(null);
  };

  const onSubmit = async (values: EditFormValues) => {
    if (!editingApplication) {
      return;
    }

    const redirectUris = values.redirectUrisText
      .split('\n')
      .map((line) => line.trim())
      .filter(Boolean);

    try {
      await updateMutation.mutateAsync({
        id: editingApplication.id,
        input: {
          name: values.name,
          description: values.description,
          status: values.status,
          redirectUris,
        },
      });
      handleClose();
    } catch {
      // Error is handled by the mutation
    }
  };

  return (
    <Dialog open={isEditDialogOpen} onOpenChange={handleClose}>
      <DialogContent className='sm:max-w-lg'>
        <DialogHeader className='text-left'>
          <DialogTitle>{t('oauthApplications.dialogs.edit.title')}</DialogTitle>
          <DialogDescription>{t('oauthApplications.dialogs.edit.description')}</DialogDescription>
        </DialogHeader>

        <Form {...form}>
          <form onSubmit={form.handleSubmit(onSubmit)} className='space-y-4'>
            <FormItem>
              <FormLabel>{t('oauthApplications.fields.clientId')}</FormLabel>
              <FormControl>
                <Input value={editingApplication?.clientID ?? ''} readOnly className='font-mono' />
              </FormControl>
              <FormDescription>{t('oauthApplications.fields.clientIdHint')}</FormDescription>
            </FormItem>

            <FormField
              control={form.control}
              name='name'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('oauthApplications.fields.name')}</FormLabel>
                  <FormControl>
                    <Input {...field} />
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
                    <Input {...field} />
                  </FormControl>
                  <FormMessage />
                </FormItem>
              )}
            />

            <FormField
              control={form.control}
              name='status'
              render={({ field }) => (
                <FormItem>
                  <FormLabel>{t('common.columns.status')}</FormLabel>
                  <Select onValueChange={field.onChange} value={field.value}>
                    <FormControl>
                      <SelectTrigger>
                        <SelectValue />
                      </SelectTrigger>
                    </FormControl>
                    <SelectContent>
                      <SelectItem value='enabled'>{t('oauthApplications.status.enabled')}</SelectItem>
                      <SelectItem value='disabled'>{t('oauthApplications.status.disabled')}</SelectItem>
                    </SelectContent>
                  </Select>
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
                    <Textarea rows={4} {...field} />
                  </FormControl>
                  <FormDescription>{t('oauthApplications.fields.redirectUrisHint')}</FormDescription>
                  <FormMessage />
                </FormItem>
              )}
            />

            <DialogFooter>
              <Button type='button' variant='outline' onClick={handleClose} disabled={updateMutation.isPending}>
                {t('common.buttons.cancel')}
              </Button>
              <Button type='submit' disabled={updateMutation.isPending}>
                {updateMutation.isPending ? t('common.buttons.saving') : t('common.buttons.save')}
              </Button>
            </DialogFooter>
          </form>
        </Form>
      </DialogContent>
    </Dialog>
  );
}
