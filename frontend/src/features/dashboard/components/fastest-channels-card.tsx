'use client';

import { useTranslation } from 'react-i18next';
import { formatNumber } from '@/utils/format-number';
import { FastestPerformersCard } from './fastest-performers-card';
import { useFastestChannels, type DashboardMode } from '../data/fastest-performers';
import type { FastestChannel } from '../data/fastest-performers';

interface FastestChannelsCardProps {
  mode: DashboardMode;
}

export function FastestChannelsCard({ mode }: FastestChannelsCardProps) {
  const { t } = useTranslation();

  return (
    <FastestPerformersCard<FastestChannel>
      title={t('dashboard.cards.fastestPerformers.channels')}
      description={(totalRequests) => t('dashboard.cards.fastestPerformers.description', { type: t('dashboard.cards.fastestPerformers.channelType'), count: formatNumber(totalRequests) })}
      noDataLabel={t('dashboard.cards.fastestPerformers.noData')}
      useData={useFastestChannels}
      getName={(item) => item.channelName}
      mode={mode}
    />
  );
}