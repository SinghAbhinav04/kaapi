import type { StaffBookingsParams } from './types';

export const BOOKING_PATHS = {
  holds: '/v1/holds',
  hold: (id: string) => `/v1/holds/${encodeURIComponent(id)}`,
  bookings: '/v1/bookings',
  booking: (ref: string) => `/v1/bookings/${encodeURIComponent(ref)}`,
  mine: '/v1/me/bookings',
  staff: '/v1/staff/bookings',
  export: '/v1/staff/bookings/export',
} as const;

export const bookingKeys = {
  all: ['booking'] as const,
  detail: (ref: string) => [...bookingKeys.all, 'detail', ref] as const,
  mine: () => [...bookingKeys.all, 'mine'] as const,
  staff: (params: StaffBookingsParams) => [...bookingKeys.all, 'staff', params] as const,
};

/** While a payment is settling, the confirmation page checks back this often. */
export const BOOKING_POLL_MS = 1_500;

export const BOOKING_STALE_MS = 15_000;
