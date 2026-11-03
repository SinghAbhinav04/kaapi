'use client';

import { useQueryClient, type QueryKey } from '@tanstack/react-query';
import { useEffect } from 'react';

import { analyticsKeys } from '@/apis/analytics';
import { bookingKeys } from '@/apis/booking';
import { enquiryKeys } from '@/apis/enquiries';
import { BROWSER_BASE_URL } from '@/apis/http';
import { offersKeys } from '@/apis/offers';
import { operationsKeys } from '@/apis/operations';
import { ratesKeys } from '@/apis/rates';

import { CHANGES_PATHS, CHANGES_RETRY_MS } from './const';
import type { ChangeTopic } from './types';

/** What each topic makes stale in the staff console. */
const AFFECTED: Record<Exclude<ChangeTopic, '*'>, readonly QueryKey[]> = {
  inventory: [ratesKeys.all, analyticsKeys.all, operationsKeys.dashboard()],
  bookings: [analyticsKeys.all, bookingKeys.all, operationsKeys.dashboard()],
  offers: [offersKeys.all],
  enquiries: [analyticsKeys.all, operationsKeys.dashboard(), enquiryKeys.counts()],
};

/**
 * Live staff data: the API pushes a topic whenever the data behind it changes (a payment, a
 * hold, a price edit by someone else, a new offer), and the queries it affects refetch at once.
 * Those queries don't poll; they stay fresh until told otherwise. One stream per tab.
 */
export function useDataChanges(enabled: boolean) {
  const queryClient = useQueryClient();
  useEffect(() => {
    if (!enabled) return;
    let source: EventSource | null = null;
    let retry = 0;
    const refetch = (topic: string) => {
      const keys = topic === '*' ? Object.values(AFFECTED).flat() : (AFFECTED[topic as keyof typeof AFFECTED] ?? []);
      for (const queryKey of keys) void queryClient.invalidateQueries({ queryKey });
    };
    const open = () => {
      source = new EventSource(`${BROWSER_BASE_URL}${CHANGES_PATHS.stream}`);
      source.addEventListener('change', (e) => refetch((e as MessageEvent<string>).data));
      source.onerror = () => {
        if (source?.readyState === EventSource.CLOSED) retry = window.setTimeout(open, CHANGES_RETRY_MS);
      };
    };
    open();
    return () => {
      window.clearTimeout(retry);
      source?.close();
    };
  }, [enabled, queryClient]);
}
