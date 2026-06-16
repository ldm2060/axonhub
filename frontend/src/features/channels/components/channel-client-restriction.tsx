import { useEffect, useState } from 'react';
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

// UI-only sentinel for "no channel-level override" (nil on the wire).
// The backend enum values are lowercase (off/lenient/strict) to match the
// ChannelClientRestriction GraphQL enum.
const INHERIT = 'inherit';

export function ChannelClientRestriction({ channel, onUpdate }: ChannelClientRestrictionProps) {
  const { t } = useTranslation();

  const [value, setValue] = useState(channel?.clientRestriction ?? INHERIT);

  // Reset the displayed selection when the underlying channel changes
  // (e.g. the dialog is reopened for a different row).
  useEffect(() => {
    setValue(channel?.clientRestriction ?? INHERIT);
  }, [channel?.clientRestriction]);

  const handleChange = (next: string) => {
    setValue(next);
    if (next === INHERIT) {
      onUpdate({ clientRestriction: null, clearClientRestriction: true });
    } else {
      onUpdate({ clientRestriction: next, clearClientRestriction: false });
    }
  };

  return (
    <div className='space-y-4'>
      <div>
        <h3 className='text-lg font-medium'>{t('channels.clientRestriction.title')}</h3>
        <p className='text-muted-foreground text-sm'>{t('channels.clientRestriction.description')}</p>
      </div>

      <div className='space-y-2'>
        <Label>{t('system.retry.clientRestriction.label')}</Label>
        <Select value={value} onValueChange={handleChange}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value={INHERIT}>{t('channels.clientRestriction.inheritGlobal')}</SelectItem>
            <SelectItem value='off'>{t('channels.clientRestriction.off')}</SelectItem>
            <SelectItem value='lenient'>{t('channels.clientRestriction.lenient')}</SelectItem>
            <SelectItem value='strict'>{t('channels.clientRestriction.strict')}</SelectItem>
          </SelectContent>
        </Select>
        <p className='text-muted-foreground text-sm'>
          {value === 'lenient' && t('system.retry.clientRestriction.documentation.lenient')}
          {value === 'strict' && t('system.retry.clientRestriction.documentation.strict')}
          {value === 'off' && t('system.retry.clientRestriction.documentation.off')}
        </p>
      </div>
    </div>
  );
}
