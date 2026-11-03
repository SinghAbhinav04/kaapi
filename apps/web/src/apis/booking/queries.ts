import { keepPreviousData, queryOptions } from '@tanstack/react-query';

import { bookingApi } from './booking.api';
import { BOOKING_POLL_MS, BOOKING_STALE_MS, bookingKeys } from './const';
import type { BookingStatus, StaffBookingsParams } from './types';

/** Statuses that can still change on their own (the payment webhook hasn't landed). */
const SETTLING: ReadonlySet<BookingStatus> = new Set(['PENDING_PAYMENT']);

export const bookingQueries = {
  // Polls only while the payment is settling, then stops.
  detail: (ref: string, holdToken?: string) =>
    queryOptions({
      queryKey: bookingKeys.detail(ref),
      queryFn: ({ signal }) => bookingApi.getBooking(ref, holdToken, signal),
      staleTime: BOOKING_STALE_MS,
      refetchInterval: (query) => (SETTLING.has(query.state.data?.booking.status as BookingStatus) ? BOOKING_POLL_MS : false),
    }),
  mine: () =>
    queryOptions({
      queryKey: bookingKeys.mine(),
      queryFn: ({ signal }) => bookingApi.myBookings(signal),
      staleTime: BOOKING_STALE_MS,
    }),
  staff: (params: StaffBookingsParams) =>
    queryOptions({
      queryKey: bookingKeys.staff(params),
      queryFn: ({ signal }) => bookingApi.staffBookings(params, signal),
      staleTime: BOOKING_STALE_MS,
      placeholderData: keepPreviousData,
    }),
};
