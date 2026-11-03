export type { RateNight, RateRow, RatesGridResponse, UpdateRatesRequest, UpdateRatesResponse } from '@/apis/generated/schema';

export type RatePlanCode = 'RO-FLEX' | 'BB-FLEX' | 'BB-NRF';

export interface RatesGridParams {
  from: string;
  /** Exclusive; at most 62 nights after `from`. */
  to: string;
  plan: RatePlanCode;
}
