export type { AnalyticsBreakdown, AnalyticsCount, AnalyticsResponse } from '@/apis/generated/schema';

export interface AnalyticsParams {
  /** First booked date (YYYY-MM-DD); the API defaults to 89 days before `to`. */
  from?: string;
  /** Last booked date (YYYY-MM-DD); the API defaults to today in IST. */
  to?: string;
}
