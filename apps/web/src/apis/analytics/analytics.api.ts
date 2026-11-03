import { http, type HttpClient } from '@/apis/http';

import { ANALYTICS_PATHS } from './const';
import type { AnalyticsApi } from './interface';

export function createAnalyticsApi(client: HttpClient): AnalyticsApi {
  return {
    report: (params, signal) => client.get(ANALYTICS_PATHS.report, { signal, query: { from: params.from, to: params.to } }),
  };
}

export const analyticsApi = createAnalyticsApi(http);
