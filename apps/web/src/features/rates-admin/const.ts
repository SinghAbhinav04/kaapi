import type { RatePlanCode } from '@/apis/rates';

/** Two weeks at a time, like the board. */
export const WINDOW_DAYS = 14;

export const PLAN_OPTIONS: readonly { value: RatePlanCode; label: string }[] = [
  { value: 'RO-FLEX', label: 'Room only' },
  { value: 'BB-FLEX', label: 'Breakfast' },
  { value: 'BB-NRF', label: 'Non-refundable' },
];

/** Only the room-only price is edited; the other plans are derived from it by the API. */
export const PRICE_PLAN: RatePlanCode = 'RO-FLEX';

export const MIN_STAY_OPTIONS = [
  { value: '1', label: '1' },
  { value: '2', label: '2' },
  { value: '3', label: '3' },
] as const;

/** How far ahead staff can page (the seeded rates window). */
export const MAX_WEEKS_AHEAD = 26;
