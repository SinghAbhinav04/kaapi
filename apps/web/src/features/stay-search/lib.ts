import type { AvailabilityParams } from '@/apis/catalog';
import { DEFAULT_PARTY, PARTY_LIMITS } from '@/shared/const';
import { addDays, isValidIsoDate, nightsBetween, todayIST, weekdayIndex, type IsoDate } from '@/shared/lib/dates';

import { DEFAULT_LEAD_DAYS, DEFAULT_NIGHTS, MAX_DAYS_AHEAD, MAX_NIGHTS, SATURDAY, SEARCH_PARAM } from './const';
import type { SearchParamsLike, StaySearch } from './types';

/** First Saturday at least a week away, for two nights. */
export function defaultSearch(today: IsoDate = todayIST()): StaySearch {
  let checkIn = addDays(today, DEFAULT_LEAD_DAYS);
  while (weekdayIndex(checkIn) !== SATURDAY) checkIn = addDays(checkIn, 1);
  return { checkIn, checkOut: addDays(checkIn, DEFAULT_NIGHTS), ...DEFAULT_PARTY };
}

function read(params: SearchParamsLike, key: string): string | undefined {
  if (params instanceof URLSearchParams) return params.get(key) ?? undefined;
  const v = params[key];
  return Array.isArray(v) ? v[0] : v;
}

function clampInt(raw: string | undefined, fallback: number, min: number, max: number): number {
  const n = Number.parseInt(raw ?? '', 10);
  return Number.isFinite(n) ? Math.min(max, Math.max(min, n)) : fallback;
}

/**
 * Reads a search from the URL. Anything missing or impossible (past dates, check-out
 * before check-in, absurd party sizes) falls back to a sensible default instead of erroring.
 */
export function parseSearch(params: SearchParamsLike, today: IsoDate = todayIST()): StaySearch {
  const inRaw = read(params, SEARCH_PARAM.checkIn);
  const outRaw = read(params, SEARCH_PARAM.checkOut);
  const dates = { checkIn: defaultSearch(today).checkIn, checkOut: defaultSearch(today).checkOut };
  if (isValidIsoDate(inRaw) && inRaw >= today && nightsBetween(today, inRaw) <= MAX_DAYS_AHEAD) {
    const nights = isValidIsoDate(outRaw) ? nightsBetween(inRaw, outRaw) : 0;
    dates.checkIn = inRaw;
    dates.checkOut = nights >= 1 && nights <= MAX_NIGHTS ? (outRaw as IsoDate) : addDays(inRaw, DEFAULT_NIGHTS);
  }
  return {
    ...dates,
    adults: clampInt(read(params, SEARCH_PARAM.adults), DEFAULT_PARTY.adults, PARTY_LIMITS.adults.min, PARTY_LIMITS.adults.max),
    children: clampInt(read(params, SEARCH_PARAM.children), DEFAULT_PARTY.children, PARTY_LIMITS.children.min, PARTY_LIMITS.children.max),
    rooms: clampInt(read(params, SEARCH_PARAM.rooms), DEFAULT_PARTY.rooms, PARTY_LIMITS.rooms.min, PARTY_LIMITS.rooms.max),
  };
}

export function searchToQuery(search: StaySearch): string {
  const q = new URLSearchParams({
    [SEARCH_PARAM.checkIn]: search.checkIn,
    [SEARCH_PARAM.checkOut]: search.checkOut,
    [SEARCH_PARAM.adults]: String(search.adults),
  });
  if (search.children) q.set(SEARCH_PARAM.children, String(search.children));
  if (search.rooms !== 1) q.set(SEARCH_PARAM.rooms, String(search.rooms));
  return q.toString();
}

export function toAvailabilityParams(search: StaySearch, promo?: string): AvailabilityParams {
  return { ...search, promo };
}

export const nightsOf = (search: StaySearch) => nightsBetween(search.checkIn, search.checkOut);
