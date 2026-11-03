import type {
  BookingResponse,
  BookingsResponse,
  CreateBookingRequest,
  CreateHoldRequest,
  HoldResponse,
  StaffBookingsParams,
} from './types';

/** Holding rooms, turning holds into bookings, and reading bookings back. */
export interface BookingApi {
  /** Locks the nights and freezes the price for ten minutes. */
  createHold(body: CreateHoldRequest): Promise<HoldResponse>;
  releaseHold(holdId: string, holdToken: string): Promise<void>;
  /** Idempotent per hold: submitting twice returns the same booking. */
  createBooking(body: CreateBookingRequest, holdToken: string): Promise<BookingResponse>;
  /** For the browser that made it (hold token), its signed-in guest, or staff. */
  getBooking(ref: string, holdToken?: string, signal?: AbortSignal): Promise<BookingResponse>;
  myBookings(signal?: AbortSignal): Promise<BookingsResponse>;
  staffBookings(params: StaffBookingsParams, signal?: AbortSignal): Promise<BookingsResponse>;
  /** The day's confirmed and cancelled bookings as CSV, for the reservation system. */
  exportDay(date: string): Promise<string>;
}
