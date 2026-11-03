'use client';

import { useEffect, useEffectEvent, useRef } from 'react';

import { COUNTDOWN_TICK_MS } from './const';

/**
 * Writes "mm:ss" left until `deadline` straight into the returned element, once a second,
 * without re-rendering React. Calls `onExpire` once when time runs out.
 */
export function useCountdown<E extends HTMLElement>(deadline: string | null, onExpire?: () => void) {
  const ref = useRef<E>(null);
  const expired = useEffectEvent(() => onExpire?.());

  useEffect(() => {
    if (!deadline) return;
    const end = new Date(deadline).getTime();
    let fired = false;
    const tick = () => {
      const left = Math.max(0, end - Date.now());
      if (ref.current) ref.current.textContent = formatMinutesSeconds(left);
      if (left === 0 && !fired) {
        fired = true;
        expired();
      }
    };
    tick();
    const id = window.setInterval(tick, COUNTDOWN_TICK_MS);
    return () => window.clearInterval(id);
  }, [deadline]);

  return ref;
}

export function formatMinutesSeconds(ms: number): string {
  const total = Math.ceil(ms / 1000);
  return `${String(Math.floor(total / 60)).padStart(2, '0')}:${String(total % 60).padStart(2, '0')}`;
}
