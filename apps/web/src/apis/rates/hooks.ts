'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { catalogKeys } from '@/apis/catalog/const';

import { ratesKeys } from './const';
import { ratesApi } from './rates.api';
import { ratesQueries } from './queries';
import { applyRateChange } from './optimistic';
import type { RatesGridParams, RatesGridResponse, UpdateRatesRequest } from './types';

export const useRatesGrid = (params: RatesGridParams) => useQuery(ratesQueries.grid(params));

/**
 * A rate change shows at once (setQueryData on every cached grid), rolls back if the API
 * refuses it, and then refetches the grid and everything guests see, which brings in the
 * derived plan prices the server computes.
 */
export function useUpdateRates() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: UpdateRatesRequest) => ratesApi.update(body),
    onMutate: async (body) => {
      await queryClient.cancelQueries({ queryKey: ratesKeys.all });
      const previous = queryClient.getQueriesData<RatesGridResponse>({ queryKey: ratesKeys.all });
      queryClient.setQueriesData<RatesGridResponse>({ queryKey: ratesKeys.all }, (grid) => grid && applyRateChange(grid, body));
      return { previous };
    },
    onError: (_error, _body, context) => {
      for (const [key, grid] of context?.previous ?? []) queryClient.setQueryData(key, grid);
    },
    onSettled: async () => {
      await queryClient.invalidateQueries({ queryKey: ratesKeys.all });
      await queryClient.invalidateQueries({ queryKey: catalogKeys.all });
    },
  });
}
