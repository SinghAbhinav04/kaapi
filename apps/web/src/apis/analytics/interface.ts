import type { AnalyticsParams, AnalyticsResponse } from './types';

/** Revenue, pace and mix for the GM and administrator. */
export interface AnalyticsApi {
  report(params: AnalyticsParams, signal?: AbortSignal): Promise<AnalyticsResponse>;
}
