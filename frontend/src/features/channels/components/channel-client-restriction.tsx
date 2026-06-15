import { useTranslation } from 'react-i18next';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';

interface Channel {
  clientRestriction?: string | null;
}

interface ChannelClientRestrictionProps {
  channel: Channel | null;
  onUpdate: (updates: Partial<{ clientRestriction: string | null; clearClientRestriction: boolean }>) => void;
}

export function ChannelClientRestriction({ channel, onUpdate }: ChannelClientRestrictionProps) {
  const { t } = useTranslation();

  const value = channel?.clientRestriction || 'INHERIT';

  return (
    <div className='space-y-4'>
      <div>
        <h3 className='text-lg font-medium'>{t('channels.clientRestriction.title')}</h3>
        <p className='text-muted-foreground text-sm'>{t('channels.clientRestriction.description')}</p>
      </div>

      <div className='space-y-2'>
        <Label>{t('system.retry.clientRestriction.label')}</Label>
        <Select
          value={value}
          onValueChange={(v) => {
            onUpdate({
              clientRestriction: v === 'INHERIT' ? null : v,
              clearClientRestriction: v === 'INHERIT',
            });
          }}
        >
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='INHERIT'>{t('channels.clientRestriction.inheritGlobal')}</SelectItem>
            <SelectItem value='OFF'>{t('channels.clientRestriction.off')}</SelectItem>
            <SelectItem value='LENIENT'>{t('channels.clientRestriction.lenient')}</SelectItem>
            <SelectItem value='STRICT'>{t('channels.clientRestriction.strict')}</SelectItem>
          </SelectContent>
        </Select>
        <p className='text-muted-foreground text-sm'>
          {value === 'LENIENT' && t('system.retry.clientRestriction.documentation.lenient')}
          {value === 'STRICT' && t('system.retry.clientRestriction.documentation.strict')}
          {value === 'OFF' && t('system.retry.clientRestriction.documentation.off')}
        </p>
      </div>
    </div>
  );
}
