import { keepPreviousData, queryOptions } from '@tanstack/react-query';

import { RATES_REFRESH_MS, RATES_STALE_MS, ratesKeys } from './const';
import { ratesApi } from './rates.api';
import type { RatesGridParams } from './types';

export const ratesQueries = {
  // Paging fortnights or switching plans keeps the old grid visible, dimmed, until the new one arrives.
  grid: (params: RatesGridParams) =>
    queryOptions({
      queryKey: ratesKeys.grid(params),
      queryFn: ({ signal }) => ratesApi.grid(params, signal),
      staleTime: RATES_STALE_MS,
      refetchInterval: RATES_REFRESH_MS,
      placeholderData: keepPreviousData,
    }),
};
