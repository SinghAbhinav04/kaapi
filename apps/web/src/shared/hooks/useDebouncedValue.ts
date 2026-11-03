'use client';

import { useEffect, useState } from 'react';

import { DEBOUNCE_MS } from './const';

/** The value, once it has stopped changing for `delay` ms (search boxes). */
export function useDebouncedValue<T>(value: T, delay: number = DEBOUNCE_MS): T {
  const [settled, setSettled] = useState(value);
  useEffect(() => {
    const id = window.setTimeout(() => setSettled(value), delay);
    return () => window.clearTimeout(id);
  }, [value, delay]);
  return settled;
}
