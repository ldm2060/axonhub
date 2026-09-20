import { z } from 'zod';
import { useQuery } from '@tanstack/react-query';
import { graphqlRequest } from '@/gql/graphql';
import { buildMySharedChannelsQuery, type ChannelListColumnVisibility } from './channels';
import { channelSchema, type Channel } from './schema';

/**
 * Channels another user shared with the signed-in user.
 *
 * This goes through the dedicated `mySharedChannels` query instead of a
 * `visibility: shared` list query: the channel privacy rule only filters by
 * visibility for callers without read_channels, so a list query either returns
 * every user's shared channels or nothing at all. The server resolves the
 * shared_with membership for the current user.
 */
export function useMySharedChannels(columnVisibility?: ChannelListColumnVisibility) {
  const query = buildMySharedChannelsQuery(columnVisibility);

  return useQuery({
    queryKey: ['mySharedChannels', query],
    queryFn: async () => {
      const data = await graphqlRequest<{ mySharedChannels: Channel[] }>(query);
      return z.array(channelSchema).parse(data.mySharedChannels ?? []);
    },
    // Same cadence as the other channel lists: keeps the live limiter / quota
    // snapshots on shared rows roughly fresh while the tab is visible.
    refetchInterval: 5000,
    refetchIntervalInBackground: false,
  });
}
