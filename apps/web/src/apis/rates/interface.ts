import type { RatesGridParams, RatesGridResponse, UpdateRatesRequest, UpdateRatesResponse } from './types';

/** The staff rate and allotment grid. Every edit is audit-logged by the API. */
export interface RatesApi {
  grid(params: RatesGridParams, signal?: AbortSignal): Promise<RatesGridResponse>;
  update(body: UpdateRatesRequest): Promise<UpdateRatesResponse>;
}
