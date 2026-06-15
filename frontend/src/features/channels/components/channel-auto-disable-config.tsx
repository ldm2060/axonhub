import { useState } from 'react';
import { useTranslation } from 'react-i18next';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { Input } from '@/components/ui/input';
import { Button } from '@/components/ui/button';
import { Trash2, Plus } from 'lucide-react';

interface AutoDisableStatus {
  status: number;
  times: number;
}

interface ChannelAutoDisableConfig {
  mode: string;
  enabled?: boolean;
  statuses?: AutoDisableStatus[];
}

interface Channel {
  autoDisableConfig?: ChannelAutoDisableConfig | null;
}

interface ChannelAutoDisableConfigProps {
  channel: Channel | null;
  onUpdate: (updates: Partial<{ autoDisableConfig: ChannelAutoDisableConfig | null; clearAutoDisableConfig: boolean }>) => void;
}

export function ChannelAutoDisableConfig({ channel, onUpdate }: ChannelAutoDisableConfigProps) {
  const { t } = useTranslation();

  const config = channel?.autoDisableConfig || { mode: 'INHERIT_GLOBAL' };
  const [mode, setMode] = useState(config.mode);
  const [enabled, setEnabled] = useState(config.enabled ?? true);
  const [statuses, setStatuses] = useState<AutoDisableStatus[]>(config.statuses || []);

  const handleModeChange = (newMode: string) => {
    setMode(newMode);

    if (newMode === 'INHERIT_GLOBAL') {
      onUpdate({ autoDisableConfig: null, clearAutoDisableConfig: true });
    } else if (newMode === 'DISABLED') {
      onUpdate({
        autoDisableConfig: { mode: 'DISABLED', enabled: false, statuses: [] },
      });
    } else if (newMode === 'CUSTOM') {
      onUpdate({
        autoDisableConfig: {
          mode: 'CUSTOM',
          enabled,
          statuses,
        },
      });
    }
  };

  const handleStatusAdd = () => {
    const newStatuses = [...statuses, { status: 401, times: 3 }];
    setStatuses(newStatuses);
    onUpdate({
      autoDisableConfig: { mode: 'CUSTOM', enabled, statuses: newStatuses },
    });
  };

  const handleStatusRemove = (index: number) => {
    const newStatuses = statuses.filter((_, i) => i !== index);
    setStatuses(newStatuses);
    onUpdate({
      autoDisableConfig: { mode: 'CUSTOM', enabled, statuses: newStatuses },
    });
  };

  const handleStatusChange = (index: number, field: 'status' | 'times', value: number) => {
    const newStatuses = [...statuses];
    newStatuses[index] = { ...newStatuses[index], [field]: value };
    setStatuses(newStatuses);
    onUpdate({
      autoDisableConfig: { mode: 'CUSTOM', enabled, statuses: newStatuses },
    });
  };

  const handleEnabledChange = (newEnabled: boolean) => {
    setEnabled(newEnabled);
    onUpdate({
      autoDisableConfig: { mode: 'CUSTOM', enabled: newEnabled, statuses },
    });
  };

  return (
    <div className='space-y-4'>
      <div>
        <h3 className='text-lg font-medium'>{t('channels.autoDisable.title')}</h3>
        <p className='text-muted-foreground text-sm'>{t('channels.autoDisable.description')}</p>
      </div>

      <div className='space-y-2'>
        <Label>{t('channels.autoDisable.mode.inheritGlobal')}</Label>
        <Select value={mode} onValueChange={handleModeChange}>
          <SelectTrigger>
            <SelectValue />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value='INHERIT_GLOBAL'>{t('channels.autoDisable.mode.inheritGlobal')}</SelectItem>
            <SelectItem value='DISABLED'>{t('channels.autoDisable.mode.disabled')}</SelectItem>
            <SelectItem value='CUSTOM'>{t('channels.autoDisable.mode.custom')}</SelectItem>
          </SelectContent>
        </Select>
      </div>

      {mode === 'CUSTOM' && (
        <>
          <div className='flex items-center space-x-2'>
            <Switch checked={enabled} onCheckedChange={handleEnabledChange} />
            <Label>{t('channels.autoDisable.enabled.label')}</Label>
          </div>

          {enabled && (
            <div className='space-y-2'>
              <Label>{t('channels.autoDisable.statuses.label')}</Label>
              <div className='space-y-2'>
                {statuses.map((status, index) => (
                  <div key={index} className='flex items-center gap-2'>
                    <Input
                      type='number'
                      placeholder={t('channels.autoDisable.statuses.statusPlaceholder')}
                      value={status.status}
                      onChange={(e) => handleStatusChange(index, 'status', parseInt(e.target.value) || 0)}
                      className='w-24'
                      min='400'
                      max='599'
                    />
                    <Input
                      type='number'
                      placeholder={t('channels.autoDisable.statuses.timesPlaceholder')}
                      value={status.times}
                      onChange={(e) => handleStatusChange(index, 'times', parseInt(e.target.value) || 0)}
                      className='w-24'
                      min='1'
                      max='100'
                    />
                    <span className='text-muted-foreground text-sm'>{t('channels.autoDisable.statuses.times')}</span>
                    <Button type='button' variant='ghost' size='icon' onClick={() => handleStatusRemove(index)}>
                      <Trash2 className='h-4 w-4' />
                    </Button>
                  </div>
                ))}
              </div>
              <Button type='button' variant='outline' size='sm' onClick={handleStatusAdd} className='mt-2'>
                <Plus className='mr-2 h-4 w-4' />
                {t('channels.autoDisable.statuses.add')}
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
