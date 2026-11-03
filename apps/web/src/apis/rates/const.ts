import type { RatesGridParams } from './types';

export const RATES_PATHS = { grid: '/v1/staff/rates' } as const;

export const ratesKeys = {
  all: ['rates'] as const,
  grid: (params: RatesGridParams) => [...ratesKeys.all, params] as const,
};

/**
 * The grid never goes stale by itself: a save updates it at once (setQueryData) and the live
 * change stream (apis/changes) refetches it whenever prices, allotment, sales or holds change.
 */
export const RATES_STALE_MS = Infinity;
