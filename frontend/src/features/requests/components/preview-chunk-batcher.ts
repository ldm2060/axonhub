type ScheduleFrame = (callback: () => void) => number;
type CancelFrame = (id: number) => void;

export function createPreviewChunkBatcher<T>(
  publish: (batch: T[]) => void,
  schedule: ScheduleFrame = requestAnimationFrame,
  cancel: CancelFrame = cancelAnimationFrame
) {
  let pending: T[] = [];
  let frameId: number | null = null;
  let disposed = false;

  const publishPending = () => {
    frameId = null;
    if (disposed || pending.length === 0) return;
    const batch = pending;
    pending = [];
    publish(batch);
  };

  return {
    push(item: T) {
      if (disposed) return;
      pending.push(item);
      if (frameId === null) frameId = schedule(publishPending);
    },
    flush() {
      if (frameId !== null) cancel(frameId);
      publishPending();
    },
    dispose() {
      disposed = true;
      pending = [];
      if (frameId !== null) cancel(frameId);
      frameId = null;
    },
  };
}
