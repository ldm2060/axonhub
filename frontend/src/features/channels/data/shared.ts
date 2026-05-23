import { useMemo } from 'react';
import { useAuthStore } from '@/stores/authStore';
import { useMe } from '@/features/auth/data/auth';
import { useQueryChannels } from './channels';
import { channelSchema } from './schema';

import type { z } from 'zod';

type Channel = z.infer<typeof channelSchema>;

export function useMySharedChannels() {
  const { user: authUser } = useAuthStore((state) => state.auth);
  const { data: meData } = useMe();
  const currentUser = meData || authUser;

  const { data, isLoading } = useQueryChannels({
    where: {
      visibility: 'shared',
      statusIn: ['enabled'],
    },
    orderBy: { field: 'CREATED_AT', direction: 'DESC' },
  });

  const sharedChannels = useMemo(() => {
    if (!currentUser?.id || !data?.edges) return [];
    const userId = Number(currentUser.id);
    return data.edges
      .map((edge) => edge.node)
      .filter((ch) => Array.isArray(ch.sharedWith) && ch.sharedWith.includes(userId));
  }, [data?.edges, currentUser?.id]);

  return { data: sharedChannels, isLoading };
}