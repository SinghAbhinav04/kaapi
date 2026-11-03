export type {
  Booking,
  BookingResponse,
  BookingsResponse,
  BookingStatus,
  CreateBookingRequest,
  CreateHoldRequest,
  Guest,
  Hold,
  HoldResponse,
} from '@/apis/generated/schema';

/** A hold plus the secret that proves this browser made it. */
export interface HoldTicket {
  holdId: string;
  token: string;
}

export interface StaffBookingsParams {
  status?: string;
  /** Check-in on or after (YYYY-MM-DD). */
  from?: string;
  /** Check-in before (YYYY-MM-DD). */
  to?: string;
  limit?: number;
}
