'use client';

import { usePathname, useRouter, useSearchParams } from 'next/navigation';
import { useCallback, useState } from 'react';

import { useInbox, type Lane } from '@/apis/enquiries';
import { useDebouncedValue } from '@/shared/hooks';

import { INBOX_LIMIT } from './const';

const LANES: readonly Lane[] = ['RESERVATIONS', 'SALES', 'GM', 'UNSORTED'];

/**
 * Lane and open enquiry live in the URL (shareable, back button works). The list is one
 * cached query for every lane this user may see, filtered here, so switching lanes is
 * instant and the sidebar counts come from the same data.
 */
export function useInboxView() {
  const params = useSearchParams();
  const router = useRouter();
  const pathname = usePathname();
  const laneParam = params.get('lane') as Lane | null;
  const lane = laneParam && LANES.includes(laneParam) ? laneParam : null;
  const open = params.get('open');
  const [search, setSearch] = useState('');
  const q = useDebouncedValue(search.trim());
  const inbox = useInbox(q ? { q, limit: INBOX_LIMIT } : { limit: INBOX_LIMIT });

  const navigate = useCallback(
    (next: { lane?: Lane | null; open?: string | null }) => {
      const p = new URLSearchParams(params.toString());
      for (const [key, value] of Object.entries(next)) {
        if (value) p.set(key, value);
        else p.delete(key);
      }
      router.replace(`${pathname}?${p.toString()}`, { scroll: false });
    },
    [params, router, pathname],
  );

  const all = inbox.data?.enquiries ?? [];
  const list = lane ? all.filter((e) => e.lane === lane) : all;
  return { lane, open, search, setSearch, inbox, all, list, navigate };
}
