import { keepPreviousData, queryOptions } from '@tanstack/react-query';

import { analyticsApi } from './analytics.api';
import { ANALYTICS_STALE_MS, analyticsKeys } from './const';
import type { AnalyticsParams } from './types';

export const analyticsQueries = {
  report: (params: AnalyticsParams) =>
    queryOptions({
      queryKey: analyticsKeys.report(params),
      queryFn: ({ signal }) => analyticsApi.report(params, signal),
      staleTime: ANALYTICS_STALE_MS,
      placeholderData: keepPreviousData, // switching range keeps the old charts, dimmed, until the new ones land
    }),
};
