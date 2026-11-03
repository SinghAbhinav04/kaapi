'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { catalogKeys } from '@/apis/catalog/const';

import { ratesKeys } from './const';
import { ratesApi } from './rates.api';
import { ratesQueries } from './queries';
import type { RatesGridParams, UpdateRatesRequest } from './types';

export const useRatesGrid = (params: RatesGridParams) => useQuery(ratesQueries.grid(params));

/** A rate change affects the grid and everything guests see, so both refetch. */
export function useUpdateRates() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: UpdateRatesRequest) => ratesApi.update(body),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ratesKeys.all });
      await queryClient.invalidateQueries({ queryKey: catalogKeys.all });
    },
  });
}
