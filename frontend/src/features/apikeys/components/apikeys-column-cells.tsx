import { Copy, Eye } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { toast } from 'sonner';
import { Button } from '@/components/ui/button';
import { useApiKeysContext } from '../context/apikeys-context';
import type { ApiKey } from '../data/schema';

export function ApiKeyCell({ apiKey, fullApiKey }: { apiKey: string; fullApiKey: ApiKey }) {
  const { t } = useTranslation();
  const { openDialog } = useApiKeysContext();

  // 显示前8个字符和后4个字符，中间用省略号
  const maskedKey = apiKey.replace(/./g, '*').slice(0, -4) + apiKey.slice(-4);

  const copyToClipboard = () => {
    navigator.clipboard.writeText(apiKey);
    toast.success(t('apikeys.messages.copied'));
  };

  const handleViewKey = () => {
    openDialog('view', fullApiKey);
  };

  return (
    <div className='flex max-w-48 items-center space-x-2'>
      <code className='bg-muted truncate rounded px-2 py-1 font-mono text-sm'>{maskedKey}</code>
      <Button variant='ghost' size='sm' onClick={handleViewKey} className='h-6 w-6 flex-shrink-0 p-0' title={t('apikeys.actions.view')}>
        <Eye className='h-3 w-3' />
      </Button>
      <Button variant='ghost' size='sm' onClick={copyToClipboard} className='h-6 w-6 flex-shrink-0 p-0' title={t('apikeys.actions.copy')}>
        <Copy className='h-3 w-3' />
      </Button>
    </div>
  );
}
