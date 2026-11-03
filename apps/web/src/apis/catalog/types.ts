export type {
  Currency,
  FxRate,
  FxRatesResponse,
  AvailabilityResponse,
  CalendarDay,
  CalendarResponse,
  Fact,
  Option as AvailabilityOption,
  Photo,
  Plan,
  PlanQuote,
  PropertyResponse,
  Quote,
  QuoteLine,
  Room,
  RoomResponse,
  RoomsResponse,
  Stay,
} from '@/apis/generated/schema';

/** A stay search. Dates are YYYY-MM-DD (IST). */
export interface AvailabilityParams {
  checkIn: string;
  checkOut: string;
  adults: number;
  children: number;
  rooms: number;
  promo?: string;
}

export interface CalendarParams {
  from: string;
  /** Exclusive; at most 42 days after `from`. */
  to: string;
  plan?: string;
}
