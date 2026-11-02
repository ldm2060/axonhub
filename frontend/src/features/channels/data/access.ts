import { extractNumberIDAsNumber } from '@/lib/utils';
import type { Channel } from './schema';

type ChannelAccessFields = Pick<Channel, 'ownerID' | 'visibility' | 'sharedWith'>;
type AccessUser = { id: string; isOwner?: boolean; isAdmin?: boolean } | null | undefined;

/**
 * Mirrors `scopes.CanRouteThroughChannel` on the server: the acting user may use
 * their own, published, or shared-with-them channels. The system owner and
 * admins may use everything; write_channels alone only grants management.
 *
 * Fork-specific (see .agent/rules/channel-access.md). The server enforces this,
 * so a drift here only affects which channels the UI offers.
 */
export function canUseChannel(channel: ChannelAccessFields, user: AccessUser): boolean {
  if (!user) return false;
  if (user.isOwner || user.isAdmin) return true;
  if (!channel.ownerID) return true;

  const userID = extractNumberIDAsNumber(user.id);
  if (extractNumberIDAsNumber(channel.ownerID) === userID) return true;
  if (channel.visibility === 'published') return true;
  if (channel.visibility === 'shared') return (channel.sharedWith ?? []).includes(userID);

  return false;
}
