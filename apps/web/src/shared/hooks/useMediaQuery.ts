'use client';

import { useCallback, useSyncExternalStore } from 'react';

import { MEDIA } from './const';

/** Subscribes to a media query without a mount-time render flash. */
export function useMediaQuery(query: string, serverValue = false): boolean {
  const subscribe = useCallback(
    (notify: () => void) => {
      const list = window.matchMedia(query);
      list.addEventListener('change', notify);
      return () => list.removeEventListener('change', notify);
    },
    [query],
  );
  return useSyncExternalStore(subscribe, () => window.matchMedia(query).matches, () => serverValue);
}

export const useReducedMotion = () => useMediaQuery(MEDIA.reducedMotion);
