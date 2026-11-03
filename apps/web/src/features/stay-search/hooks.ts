'use client';

import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { useCallback, useMemo } from 'react';

import { parseSearch, searchToQuery } from './lib';
import type { StaySearch } from './types';

/** The current stay search from the URL, and a way to change it without a scroll jump. */
export function useStaySearch() {
  const params = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const search = useMemo(() => parseSearch(params), [params]);

  const setSearch = useCallback(
    (next: StaySearch, options: { hash?: string } = {}) => {
      router.replace(`${pathname}?${searchToQuery(next)}${options.hash ?? ''}`, { scroll: false });
    },
    [router, pathname],
  );

  return { search, setSearch };
}
