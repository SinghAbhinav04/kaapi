/** URL parameter names for a stay search. */
export const SEARCH_PARAM = {
  checkIn: 'in',
  checkOut: 'out',
  adults: 'adults',
  children: 'children',
  rooms: 'rooms',
} as const;

/** With no dates chosen, suggest the first weekend at least this many days out. */
export const DEFAULT_LEAD_DAYS = 7;
export const DEFAULT_NIGHTS = 2;
/** The API refuses longer stays; the form does too. */
export const MAX_NIGHTS = 30;
/** Bookable window (matches the seeded rates). */
export const MAX_DAYS_AHEAD = 180;

export const SATURDAY = 5; // weekdayIndex: Monday = 0
