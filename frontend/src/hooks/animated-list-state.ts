export interface AnimatedListItem {
  id: string;
  createdAt: Date | string;
}

function getTimestamp(value: Date | string): number {
  return value instanceof Date ? value.getTime() : new Date(value).getTime();
}

export function reconcileAnimatedQueue<T extends AnimatedListItem>(
  queue: T[],
  incoming: T[],
  displayed: T[],
  maxSize: number
): T[] {
  if (maxSize <= 0) return [];

  const incomingIds = new Set(incoming.map(({ id }) => id));
  const displayedIds = new Set(displayed.map(({ id }) => id));
  const newestDisplayedTime = displayed.length > 0 ? getTimestamp(displayed[0].createdAt) : Number.NEGATIVE_INFINITY;
  const retained = queue.filter(({ id }) => incomingIds.has(id) && !displayedIds.has(id));
  const queuedIds = new Set(retained.map(({ id }) => id));
  const additions = incoming.filter(({ id, createdAt }) => {
    return !displayedIds.has(id) && !queuedIds.has(id) && getTimestamp(createdAt) > newestDisplayedTime;
  });
  const candidates = [...retained, ...additions].sort(
    (left, right) => getTimestamp(left.createdAt) - getTimestamp(right.createdAt)
  );

  return candidates.length <= maxSize ? candidates : candidates.slice(-maxSize);
}
