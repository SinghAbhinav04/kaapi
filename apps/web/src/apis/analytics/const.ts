import type { AnalyticsParams } from './types';

export const ANALYTICS_PATHS = {
  report: '/v1/staff/analytics',
} as const;

export const analyticsKeys = {
  all: ['analytics'] as const,
  report: (params: AnalyticsParams) => [...analyticsKeys.all, 'report', params] as const,
};

/** Reports change only as bookings arrive; a minute is fresh enough. */
export const ANALYTICS_STALE_MS = 60_000;
