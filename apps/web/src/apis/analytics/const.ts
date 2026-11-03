import type { AnalyticsParams } from './types';

export const ANALYTICS_PATHS = {
  report: '/v1/staff/analytics',
} as const;

export const analyticsKeys = {
  all: ['analytics'] as const,
  report: (params: AnalyticsParams) => [...analyticsKeys.all, 'report', params] as const,
};

/** Reports change only when bookings, inventory or enquiries do, and the change stream says when. */
export const ANALYTICS_STALE_MS = Infinity;
