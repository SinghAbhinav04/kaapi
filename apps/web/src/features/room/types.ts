import type { IsoDate } from '@/shared/lib/dates';

/** How a calendar day looks and behaves. */
export type DayState =
  | 'past' // before today
  | 'closed' // stop-sell or sold out online
  | 'night' // a night of the selected stay
  | 'checkout' // the morning the stay ends
  | 'pending' // check-in picked, waiting for check-out
  | 'open';

export interface CalendarCell {
  date: IsoDate;
  day: number;
  state: DayState;
  /** "17.5" (₹ thousands, room only, before GST), or "—". */
  price: string;
  left: number | null;
  season: boolean;
  minStay: number;
  /** Arrivals not allowed this day (it can still be a stay night or a check-out). */
  noArrival: boolean;
  label: string;
}
