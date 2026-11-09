import type { RateNight, RatesGridResponse, UpdateRatesRequest } from './types';

/** The plan whose price staff edit; other plans are derived from it on the server. */
const ROOM_ONLY_PLAN = 'RO-FLEX';
const WEEKDAY_CODES = ['SUN', 'MON', 'TUE', 'WED', 'THU', 'FRI', 'SAT'] as const;

/**
 * The grid as it will look once an edit is saved, so the screen changes the moment staff
 * press save (setQueryData). The server's answer then replaces it: derived plan prices and
 * anything another person changed arrive with the refetch. `to` is exclusive, as on the API.
 */
export function applyRateChange(grid: RatesGridResponse, change: UpdateRatesRequest): RatesGridResponse {
  const days = change.weekdays?.length ? new Set<string>(change.weekdays) : null;
  const weekday = (date: string) => WEEKDAY_CODES[new Date(`${date}T00:00:00Z`).getUTCDay()] ?? '';
  const touches = (date: string) => date >= change.from && date < change.to && (!days || days.has(weekday(date)));

  const patch = (n: RateNight): RateNight => {
    if (!touches(n.date)) return n;
    const next = { ...n };
    if (change.room_only_price_paise !== undefined && grid.plan === ROOM_ONLY_PLAN) next.price_paise = change.room_only_price_paise;
    if (change.allotment !== undefined) {
      next.allotment = change.allotment;
      next.left = Math.max(change.allotment - n.sold - n.held, 0);
    }
    if (change.stop_sell !== undefined) {
      next.stop_sell = change.stop_sell;
      next.stop_sell_reason = change.stop_sell ? (change.stop_sell_reason ?? null) : null; // as the server stores it
    }
    if (change.min_stay !== undefined) next.min_stay = change.min_stay;
    if (change.no_arrival !== undefined) next.no_arrival = change.no_arrival;
    return next;
  };

  return {
    ...grid,
    rows: grid.rows.map((row) => (row.room_type_id === change.room_type_id ? { ...row, nights: row.nights.map(patch) } : row)),
  };
}
