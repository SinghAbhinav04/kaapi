import { HEADER, http, type HttpClient } from '@/apis/http';

import { BOOKING_PATHS } from './const';
import type { BookingApi } from './interface';

const withToken = (token?: string) => (token ? { [HEADER.holdToken]: token } : undefined);

export function createBookingApi(client: HttpClient): BookingApi {
  return {
    createHold: (body) => client.post(BOOKING_PATHS.holds, body),
    releaseHold: (holdId, token) => client.delete(BOOKING_PATHS.hold(holdId), { headers: withToken(token) }),
    createBooking: (body, token) => client.post(BOOKING_PATHS.bookings, body, { headers: withToken(token) }),
    getBooking: (ref, token, signal) => client.get(BOOKING_PATHS.booking(ref), { signal, headers: withToken(token) }),
    myBookings: (signal) => client.get(BOOKING_PATHS.mine, { signal }),
    staffBookings: (params, signal) =>
      client.get(BOOKING_PATHS.staff, {
        signal,
        query: { status: params.status, from: params.from, to: params.to, limit: params.limit },
      }),
    exportDay: (date) => client.get(BOOKING_PATHS.export, { query: { date }, responseType: 'text' }),
  };
}

export const bookingApi = createBookingApi(http);
