'use client';

import { useMemo, useState } from 'react';
import { useShareableUsers } from '@/gql/sharing';
import { CarFront, Plus, Trash2 } from 'lucide-react';
import { useTranslation } from 'react-i18next';
import { extractNumberIDAsNumber, formatUserName } from '@/lib/utils';
import { Badge } from '@/components/ui/badge';
import { Button } from '@/components/ui/button';
import { Input } from '@/components/ui/input';
import { Label } from '@/components/ui/label';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select';
import { Switch } from '@/components/ui/switch';
import { AutoCompleteSelect } from '@/components/auto-complete-select';
import { parseQuotaLimits } from '@/features/system/data/quotas';
import { useUsageMonitorChannels } from '@/features/usage-monitor/data/usage-monitor';
import { useChannelPoolStatus, POOL_STATUS } from '../data/pooling';
import type { ChannelPoolingConfig, ChannelPoolingMember } from '../data/schema';

// A pool binds exactly one estimated window (fixed-percentage shares, one
// window per channel — splitting several windows at once gets ungovernable).
const POOL_WINDOW_OPTIONS = [
  { value: '5h', labelKey: 'quota.window.5h' },
  { value: 'daily', labelKey: 'quota.window.daily' },
  { value: 'weekly', labelKey: 'quota.window.weekly' },
  { value: 'monthly', labelKey: 'quota.window.monthly' },
] as const;

// Mirrors the backend alias map: Claude's 7d pool is the weekly pool, 30d the
// monthly one, while the pooling window only offers the estimate windows.
const WINDOW_ALIASES: Record<string, string[]> = {
  '5h': ['5h'],
  daily: ['daily'],
  weekly: ['weekly', '7d'],
  monthly: ['monthly', '30d'],
};

interface ChannelPoolingSettingsProps {
  config: ChannelPoolingConfig | null;
  onChange: (config: ChannelPoolingConfig | null) => void;
  channelId?: string;
}

export function ChannelPoolingSettings({ config, onChange, channelId }: ChannelPoolingSettingsProps) {
  const { t, i18n } = useTranslation();
  const { data: monitors } = useUsageMonitorChannels();
  const enabled = config?.enabled === true;

  const [selectedUserId, setSelectedUserId] = useState('');
  const [newSharePct, setNewSharePct] = useState('');

  const { data: shareableUsers = [], isLoading: isLoadingUsers } = useShareableUsers({ disableAutoFetch: !enabled });
  // Live per-member used/status of the SAVED config; while editing, quota
  // previews recompute locally and live figures only fill the "used" column.
  const { data: poolStatus } = useChannelPoolStatus(channelId, enabled);

  const monitorOptions = useMemo(() => (monitors ?? []).map((m) => ({ value: m.id, label: m.name })), [monitors]);

  // The current estimate for the configured window, straight from the monitor's
  // persisted quota limits — the same figure the battery popover prices.
  const poolTotal = useMemo(() => {
    if (!config?.enabled || !config.monitorId) return null;
    const monitor = (monitors ?? []).find((m) => extractNumberIDAsNumber(m.id) === config.monitorId);
    if (!monitor) return null;
    const limits = parseQuotaLimits({ _limits: (monitor.quotaLimits as { items?: unknown } | null)?.items });
    const labels = WINDOW_ALIASES[config.window] ?? [config.window];
    const match = limits.find(
      (l) => l.window != null && labels.includes(l.window) && l.periodQuota != null && l.periodQuota > 0 && l.periodStart
    );
    return match ? { total: match.periodQuota as number, nextResetAt: match.nextResetAt ?? null } : null;
  }, [config?.enabled, config?.monitorId, config?.window, monitors]);

  const formatMoney = (val: number) =>
    t('currencies.format', {
      val,
      currency: 'USD',
      locale: i18n.language.startsWith('zh') ? 'zh-CN' : 'en-US',
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    });

  const memberName = (userId: number) => {
    const live = (poolStatus?.members ?? []).find((m) => m.userId === userId)?.user;
    if (live) return formatUserName(live.firstName, live.lastName) || live.email || `#${userId}`;
    const known = shareableUsers.find((u) => extractNumberIDAsNumber(u.id) === userId);
    if (known) return formatUserName(known.firstName, known.lastName) || known.email;
    return `#${userId}`;
  };

  const memberEmail = (userId: number) => {
    const live = (poolStatus?.members ?? []).find((m) => m.userId === userId)?.user;
    if (live?.email) return live.email;
    return shareableUsers.find((u) => extractNumberIDAsNumber(u.id) === userId)?.email ?? '';
  };

  const shareSum = (config?.members ?? []).reduce((sum, m) => sum + (m.sharePct || 0), 0);
  const shareOverflow = shareSum > 100 + 1e-9;

  const patch = (partial: Partial<ChannelPoolingConfig>) => {
    onChange({ enabled: false, monitorId: 0, window: 'monthly', members: [], ...config, ...partial });
  };

  const handleEnabledChange = (on: boolean) => {
    if (on) {
      // First enable starts from a sensible default: monthly window, no members.
      onChange(config ? { ...config, enabled: true } : { enabled: true, monitorId: 0, window: 'monthly', members: [] });
    } else if (config) {
      // Disabling keeps the stored members so re-enabling restores them.
      onChange({ ...config, enabled: false });
    }
  };

  const handleAddMember = () => {
    if (!config || !selectedUserId) return;
    const userId = extractNumberIDAsNumber(selectedUserId);
    if (!userId || config.members.some((m) => m.userId === userId)) return;
    const sharePct = Number(newSharePct);
    if (!Number.isFinite(sharePct) || sharePct <= 0) return;
    const members = [...config.members, { userId, sharePct } satisfies ChannelPoolingMember];
    onChange({ ...config, members });
    setSelectedUserId('');
    setNewSharePct('');
  };

  const handleRemoveMember = (userId: number) => {
    if (!config) return;
    onChange({ ...config, members: config.members.filter((m) => m.userId !== userId) });
  };

  const handleShareChange = (userId: number, raw: string) => {
    if (!config) return;
    const sharePct = Number(raw);
    const members = config.members.map((m) => (m.userId === userId ? { ...m, sharePct: Number.isFinite(sharePct) ? sharePct : 0 } : m));
    onChange({ ...config, members });
  };

  const memberOptionPool = new Set((config?.members ?? []).map((m) => m.userId));
  const userOptions = shareableUsers
    .filter((u) => !memberOptionPool.has(extractNumberIDAsNumber(u.id)))
    .map((u) => ({
      value: u.id,
      label: `${formatUserName(u.firstName, u.lastName) || u.email} (${u.email})`,
    }));

  const liveMemberByUserId = useMemo(() => {
    const map = new Map<number, { used: number; status: string }>();
    for (const m of poolStatus?.members ?? []) {
      map.set(m.userId, { used: m.used, status: m.status });
    }
    return map;
  }, [poolStatus]);

  return (
    <div className='space-y-2'>
      <div className='flex items-center gap-2'>
        <Switch checked={enabled} onCheckedChange={handleEnabledChange} />
        <Label className='text-sm'>{t('channels.pooling.enabled')}</Label>
      </div>
      <p className='text-muted-foreground text-xs'>{t('channels.pooling.description')}</p>

      {enabled && (
        <div className='space-y-3 rounded-md border p-3'>
          <div className='grid grid-cols-2 gap-2'>
            <div className='space-y-1.5'>
              <Label className='text-xs'>{t('channels.pooling.monitor')}</Label>
              <Select value={config?.monitorId ? String(config.monitorId) : ''} onValueChange={(v) => patch({ monitorId: Number(v) })}>
                <SelectTrigger className='h-8'>
                  <SelectValue placeholder={t('channels.pooling.monitorPlaceholder')} />
                </SelectTrigger>
                <SelectContent>
                  {monitorOptions.map((m) => (
                    <SelectItem key={m.value} value={String(extractNumberIDAsNumber(m.value))}>
                      {m.label}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
            <div className='space-y-1.5'>
              <Label className='text-xs'>{t('channels.pooling.window')}</Label>
              <Select value={config?.window || undefined} onValueChange={(v) => patch({ window: v })}>
                <SelectTrigger className='h-8'>
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {POOL_WINDOW_OPTIONS.map((w) => (
                    <SelectItem key={w.value} value={w.value}>
                      {t(w.labelKey)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          </div>

          {/* Pool total preview */}
          <div className='bg-muted/50 flex items-center gap-2 rounded-md p-2'>
            <CarFront className='text-muted-foreground h-4 w-4 shrink-0' />
            <div className='min-w-0 flex-1'>
              {poolTotal ? (
                <p className='truncate text-xs'>
                  <span className='text-muted-foreground'>{t('channels.pooling.totalEstimate')}</span>
                  <span className='ml-1.5 font-medium tabular-nums'>{formatMoney(poolTotal.total)}</span>
                  {poolTotal.nextResetAt && (
                    <span className='text-muted-foreground ml-1.5'>
                      · {t('channels.pooling.totalResets')} {new Date(poolTotal.nextResetAt).toLocaleString()}
                    </span>
                  )}
                </p>
              ) : (
                <p className='text-xs text-amber-500'>{t('channels.pooling.totalUnavailable')}</p>
              )}
            </div>
          </div>

          {/* Member editor */}
          <div className='space-y-2'>
            <Label className='text-xs'>{t('channels.pooling.members')}</Label>
            <div className='flex gap-2'>
              <div className='min-w-0 flex-1'>
                <AutoCompleteSelect
                  selectedValue={selectedUserId}
                  onSelectedValueChange={setSelectedUserId}
                  items={userOptions}
                  isLoading={isLoadingUsers}
                  emptyMessage={t('channels.pooling.noUsers')}
                  placeholder={t('channels.pooling.searchUsers')}
                />
              </div>
              <Input
                type='number'
                min={0}
                max={100}
                step='0.1'
                value={newSharePct}
                onChange={(e) => setNewSharePct(e.target.value)}
                placeholder='%'
                className='h-9 w-20'
              />
              <Button type='button' size='sm' className='h-9' onClick={handleAddMember} disabled={!selectedUserId || !newSharePct}>
                <Plus className='h-4 w-4' />
              </Button>
            </div>

            {(config?.members.length ?? 0) === 0 && <p className='text-muted-foreground text-sm'>{t('channels.pooling.empty')}</p>}

            {(config?.members ?? []).map((member) => {
              const live = liveMemberByUserId.get(member.userId);
              const previewQuota = poolTotal ? (poolTotal.total * member.sharePct) / 100 : null;
              return (
                <div key={member.userId} className='border-border flex items-center gap-2 rounded-md border p-2'>
                  <div className='min-w-0 flex-1'>
                    <p className='truncate text-sm font-medium'>{memberName(member.userId)}</p>
                    {memberEmail(member.userId) && <p className='text-muted-foreground truncate text-xs'>{memberEmail(member.userId)}</p>}
                  </div>
                  <div className='text-right'>
                    <Input
                      type='number'
                      min={0}
                      max={100}
                      step='0.1'
                      value={String(member.sharePct)}
                      onChange={(e) => handleShareChange(member.userId, e.target.value)}
                      className='h-7 w-20 text-xs tabular-nums'
                    />
                  </div>
                  <div className='w-24 text-right text-xs tabular-nums'>
                    {previewQuota != null ? (
                      <span className='font-medium'>{formatMoney(previewQuota)}</span>
                    ) : (
                      <span className='text-muted-foreground'>—</span>
                    )}
                    {live && (
                      <p className='text-muted-foreground text-[11px] tabular-nums'>
                        {t('channels.pooling.usedShort')} {formatMoney(live.used)}
                      </p>
                    )}
                  </div>
                  {live && (
                    <Badge
                      variant='outline'
                      className={
                        live.status === POOL_STATUS.paused
                          ? 'border-red-500/20 bg-red-500/10 text-xs text-red-500'
                          : live.status === POOL_STATUS.warning
                            ? 'border-amber-500/20 bg-amber-500/10 text-xs text-amber-500'
                            : live.status === POOL_STATUS.available
                              ? 'border-green-500/20 bg-green-500/10 text-xs text-green-500'
                              : 'text-muted-foreground border-border text-xs'
                      }
                    >
                      {t(`channels.pooling.status.${live.status}`)}
                    </Badge>
                  )}
                  <Button
                    type='button'
                    variant='ghost'
                    size='icon'
                    className='text-muted-foreground hover:text-destructive h-7 w-7 shrink-0'
                    onClick={() => handleRemoveMember(member.userId)}
                  >
                    <Trash2 className='h-3.5 w-3.5' />
                  </Button>
                </div>
              );
            })}
          </div>

          <p className={`text-xs tabular-nums ${shareOverflow ? 'text-red-500' : 'text-muted-foreground'}`}>
            {t('channels.pooling.shareSum', { sum: shareSum.toFixed(1), reserved: Math.max(0, 100 - shareSum).toFixed(1) })}
            {shareOverflow && ` · ${t('channels.pooling.shareOverflow')}`}
          </p>
        </div>
      )}
    </div>
  );
}
