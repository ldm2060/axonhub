// 职责: 集中自动停用/可用性对话框共享的 cron 预设、时区选项与偏移量计算。
// 关联文件:
//   frontend/src/features/channels/components/api-key-auto-disable-rules-editor.tsx
//   frontend/src/features/channels/components/channels-availability-dialog.tsx
// 遵循的 Rules:
//   .agents/rules/frontend-general.md
// 注意事项:
//   仅导出常量与纯函数；组件留在各自的 tsx 文件里以满足 react-refresh 规则。

export const CRON_PRESETS = ['0 * * * *', '0 */6 * * *', '0 0 * * *', '0 0 * * 1', '0 0 1 * *'] as const;

export const CRON_PRESET_LABEL_KEYS: Record<string, string> = {
  '0 * * * *': 'channels.dialogs.availability.cronPresets.hourly',
  '0 */6 * * *': 'channels.dialogs.availability.cronPresets.every6h',
  '0 0 * * *': 'channels.dialogs.availability.cronPresets.daily',
  '0 0 * * 1': 'channels.dialogs.availability.cronPresets.weekly',
  '0 0 1 * *': 'channels.dialogs.availability.cronPresets.monthly',
};

export const TIMEZONE_OPTIONS = [
  'UTC',
  'Asia/Shanghai',
  'Asia/Hong_Kong',
  'Asia/Tokyo',
  'Asia/Seoul',
  'Asia/Singapore',
  'Asia/Kolkata',
  'Asia/Dubai',
  'Europe/London',
  'Europe/Paris',
  'Europe/Moscow',
  'America/Sao_Paulo',
  'America/New_York',
  'America/Chicago',
  'America/Denver',
  'America/Los_Angeles',
  'Australia/Sydney',
  'Pacific/Auckland',
] as const;

export function zoneOffsetLabel(timeZone: string): string {
  try {
    const parts = new Intl.DateTimeFormat('en-US', { timeZone, timeZoneName: 'longOffset' }).formatToParts(new Date());
    return parts.find((part) => part.type === 'timeZoneName')?.value ?? '';
  } catch {
    return '';
  }
}
