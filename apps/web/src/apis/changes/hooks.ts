'use client';

import { useQueryClient, type QueryKey } from '@tanstack/react-query';

import { analyticsKeys } from '@/apis/analytics';
import { bookingKeys } from '@/apis/booking';
import { enquiryKeys } from '@/apis/enquiries';
import { useEventStream } from '@/apis/http';
import { notificationKeys } from '@/apis/notifications';
import { offersKeys } from '@/apis/offers';
import { operationsKeys } from '@/apis/operations';
import { ratesKeys } from '@/apis/rates';

import { CHANGES_PATHS } from './const';
import type { ChangeTopic } from './types';

/** What each topic makes stale in the staff console. */
const AFFECTED: Record<Exclude<ChangeTopic, '*'>, readonly QueryKey[]> = {
  inventory: [ratesKeys.all, analyticsKeys.all, operationsKeys.dashboard()],
  bookings: [analyticsKeys.all, bookingKeys.all, operationsKeys.dashboard()],
  offers: [offersKeys.all],
  enquiries: [analyticsKeys.all, operationsKeys.dashboard(), enquiryKeys.counts()],
  notifications: [notificationKeys.all],
};

/**
 * Live staff data: the API pushes a topic whenever the data behind it changes (a payment, a
 * hold, a price edit by someone else, a new offer), and the queries it affects refetch at once.
 * Those queries don't poll; they stay fresh until told otherwise. One stream per tab.
 */
export function useDataChanges(enabled: boolean) {
  const queryClient = useQueryClient();
  const refetch = (topic: string) => {
    const keys = topic === '*' ? Object.values(AFFECTED).flat() : (AFFECTED[topic as keyof typeof AFFECTED] ?? []);
    for (const queryKey of keys) void queryClient.invalidateQueries({ queryKey });
  };
  useEventStream(CHANGES_PATHS.stream, { change: refetch }, () => refetch('*'), enabled);
}
