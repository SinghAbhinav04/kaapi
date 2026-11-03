import type { RatesGridParams } from './types';

export const RATES_PATHS = { grid: '/v1/staff/rates' } as const;

export const ratesKeys = {
  all: ['rates'] as const,
  grid: (params: RatesGridParams) => [...ratesKeys.all, params] as const,
};

/** Holds and bookings change "left" constantly, so the grid refreshes while open. */
export const RATES_REFRESH_MS = 30_000;
export const RATES_STALE_MS = 15_000;
