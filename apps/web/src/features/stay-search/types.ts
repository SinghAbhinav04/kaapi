import type { IsoDate } from '@/shared/lib/dates';

/** What the guest is looking for. Lives in the URL, so it survives reloads and sharing. */
export interface StaySearch {
  checkIn: IsoDate;
  checkOut: IsoDate;
  adults: number;
  children: number;
  rooms: number;
}

/** Next.js page searchParams, or the browser's URLSearchParams. */
export type SearchParamsLike = URLSearchParams | Record<string, string | string[] | undefined>;
