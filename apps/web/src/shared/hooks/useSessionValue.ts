'use client';

import { useMemo, useSyncExternalStore } from 'react';

const noSubscribe = () => () => {};

/**
 * Reads a JSON value from sessionStorage without a hydration mismatch: the server render
 * and first client render see `undefined` ("not read yet"), then the stored value or null.
 */
export function useSessionValue<T>(key: string): T | null | undefined {
  const raw = useSyncExternalStore(
    noSubscribe,
    () => {
      try {
        return window.sessionStorage.getItem(key);
      } catch {
        return null;
      }
    },
    () => undefined,
  );
  return useMemo(() => {
    if (raw === undefined) return undefined;
    if (raw === null) return null;
    try {
      return JSON.parse(raw) as T;
    } catch {
      return null;
    }
  }, [raw]);
}
