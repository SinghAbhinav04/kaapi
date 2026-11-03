import { http, type HttpClient } from '@/apis/http';

import { RATES_PATHS } from './const';
import type { RatesApi } from './interface';

export function createRatesApi(client: HttpClient): RatesApi {
  return {
    grid: (params, signal) => client.get(RATES_PATHS.grid, { signal, query: { from: params.from, to: params.to, plan: params.plan } }),
    update: (body) => client.put(RATES_PATHS.grid, body),
  };
}

export const ratesApi = createRatesApi(http);
