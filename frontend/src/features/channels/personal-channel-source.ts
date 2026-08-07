export type PersonalChannelSource = 'public' | 'shared' | 'mine';

export type PersonalChannelBaseWhere = Record<string, string | string[] | boolean>;

interface PersonalChannelOwnerItem {
  status?: string;
  ownerID?: string | number | null;
}

interface PersonalChannelFilterItem {
  name?: string;
  type?: string;
  tags?: string[] | null;
  supportedModels?: string[];
}

interface PersonalChannelRowFilters {
  nameContainsFold?: string;
  typeIn?: string[];
  hasTag?: string;
  model?: string;
}

export function isPersonalChannelSourceReadOnly(source: PersonalChannelSource) {
  return source !== 'mine';
}

export function buildPersonalChannelWhere(
  source: PersonalChannelSource,
  currentUserId: string | undefined,
  baseWhere: PersonalChannelBaseWhere
) {
  const where: PersonalChannelBaseWhere = { ...baseWhere };

  if (source === 'mine') {
    if (!where.statusIn) {
      where.statusIn = ['enabled', 'disabled'];
    }
    if (currentUserId) {
      where.ownerID = currentUserId;
    }
    return where;
  }

  where.statusIn = ['enabled'];
  // Do NOT set ownerIDNEQ here — SQL NULL != value yields UNKNOWN, which
  // excludes channels with owner_id = NULL.  Instead, filter the current
  // user's own channels on the client side (see filterOwnedPersonalChannels).
  where.visibility = source === 'public' ? 'published' : 'shared';
  return where;
}

export function filterOwnedPersonalChannels<T extends PersonalChannelOwnerItem>(channels: T[], currentUserId: string | number | undefined) {
  const currentUserIdText = currentUserId == null ? undefined : String(currentUserId);

  return channels.filter((channel) => {
    if (!currentUserIdText) {
      return true;
    }
    return String(channel.ownerID ?? '') !== currentUserIdText;
  });
}

export function filterSharedPersonalChannels<T extends PersonalChannelOwnerItem>(
  channels: T[],
  currentUserId: string | number | undefined
) {
  const currentUserIdText = currentUserId == null ? undefined : String(currentUserId);

  return channels.filter((channel) => {
    if (channel.status !== 'enabled') {
      return false;
    }
    if (!currentUserIdText) {
      return true;
    }
    return String(channel.ownerID ?? '') !== currentUserIdText;
  });
}

export function filterPersonalChannelRows<T extends PersonalChannelFilterItem>(channels: T[], filters: PersonalChannelRowFilters) {
  const name = filters.nameContainsFold?.trim().toLocaleLowerCase();
  const typeIn = filters.typeIn ?? [];
  const tag = filters.hasTag?.trim();
  const model = filters.model?.trim().toLocaleLowerCase();

  return channels.filter((channel) => {
    if (name && !channel.name?.toLocaleLowerCase().includes(name)) {
      return false;
    }
    if (typeIn.length > 0 && !typeIn.includes(channel.type ?? '')) {
      return false;
    }
    if (tag && !(channel.tags ?? []).includes(tag)) {
      return false;
    }
    if (model && !channel.supportedModels?.some((supportedModel) => supportedModel.toLocaleLowerCase().includes(model))) {
      return false;
    }
    return true;
  });
}
