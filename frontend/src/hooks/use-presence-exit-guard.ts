import * as React from 'react';

/**
 * Radix keeps a closing component mounted until the browser fires the
 * `animationend` event of its exit animation. When frame production stops
 * (occluded or backgrounded window, renderer throttling), that event never
 * arrives: the dialog overlay and its scroll lock (`pointer-events: none` on
 * body) stay alive and the whole page becomes unclickable until reload.
 *
 * This hook arms a one-shot timer after a controlled component closes and
 * re-dispatches the exit `animationend` on any element that is still stuck in
 * the closed state, letting Radix Presence finish its unmount. It is a no-op
 * when the exit animation completed normally.
 *
 * Returns an `arm` callback for uncontrolled components: call it when the
 * component closes (e.g. from a wrapped `onOpenChange`).
 */
export function usePresenceExitGuard(open: boolean | undefined, selector: string, delayMs = 600) {
  const prevOpenRef = React.useRef(open);
  const timerRef = React.useRef<number | null>(null);

  const nudgeStuckElements = React.useCallback(() => {
    document.querySelectorAll<HTMLElement>(selector).forEach((el) => {
      if (el.getAttribute('data-state') !== 'closed') return;
      const first = getComputedStyle(el).animationName.split(',')[0].trim();
      if (first && first !== 'none') {
        el.dispatchEvent(new AnimationEvent('animationend', { animationName: first, bubbles: true, cancelable: false }));
      }
    });
  }, [selector]);

  const arm = React.useCallback(() => {
    if (timerRef.current !== null) window.clearTimeout(timerRef.current);
    timerRef.current = window.setTimeout(() => {
      timerRef.current = null;
      nudgeStuckElements();
    }, delayMs);
  }, [nudgeStuckElements, delayMs]);

  React.useEffect(() => {
    const wasOpen = prevOpenRef.current;
    prevOpenRef.current = open;
    // Controlled components: arm on an open -> closed transition.
    if (wasOpen === true && open === false) arm();
  }, [open, arm]);

  React.useEffect(
    () => () => {
      if (timerRef.current !== null) window.clearTimeout(timerRef.current);
    },
    []
  );

  return arm;
}
