'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { catalogKeys } from '@/apis/catalog';

import { offersKeys } from './const';
import { offersApi } from './offers.api';
import { offersQueries } from './queries';
import type { CampaignRequest, OfferRequest } from './types';

export const useOffers = () => useQuery(offersQueries.list());
export const useCampaigns = () => useQuery(offersQueries.campaigns());

/** An offer changes what guests are quoted, so cached prices are refreshed too. */
function useOfferMutation<V, R>(fn: (vars: V) => Promise<R>) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: fn,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: offersKeys.all });
      void queryClient.invalidateQueries({ queryKey: catalogKeys.all });
    },
  });
}

export const useCreateOffer = () => useOfferMutation((body: OfferRequest) => offersApi.create(body));
export const useUpdateOffer = () => useOfferMutation(({ id, body }: { id: string; body: OfferRequest }) => offersApi.update(id, body));

export function useCreateCampaign() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CampaignRequest) => offersApi.createCampaign(body),
    onSuccess: () => void queryClient.invalidateQueries({ queryKey: offersKeys.campaigns() }),
  });
}
