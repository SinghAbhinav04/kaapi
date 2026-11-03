'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { enquiryKeys } from './const';
import { enquiriesApi } from './enquiries.api';
import { enquiryQueries } from './queries';
import type {
  CorrectEnquiryRequest,
  CreateEnquiryRequest,
  EnquiriesResponse,
  EnquiryResponse,
  EnquiryStatus,
  InboundMessageRequest,
  InboxParams,
  RouteRequest,
} from './types';

export const useInbox = (params: InboxParams) => useQuery(enquiryQueries.list(params));

export const useEnquiry = (ref: string | null) => useQuery({ ...enquiryQueries.detail(ref ?? ''), enabled: ref !== null });

export const useCreateEnquiry = () => useMutation({ mutationFn: (body: CreateEnquiryRequest) => enquiriesApi.create(body) });

export function useInboundMessage() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: InboundMessageRequest) => enquiriesApi.inbound(body),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: enquiryKeys.lists() }),
  });
}

/**
 * Every staff action returns the updated enquiry: it replaces the cached detail at once,
 * and the lists refetch in the background (lane, status or "draft ready" may have changed).
 */
function useEnquiryAction<V>(ref: string, action: (vars: V) => Promise<EnquiryResponse>) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: action,
    onSuccess: (data) => queryClient.setQueryData(enquiryKeys.detail(ref), data),
    onSettled: () => queryClient.invalidateQueries({ queryKey: enquiryKeys.lists() }),
  });
}

/**
 * Re-routing is optimistic: the row leaves the current lane immediately (one click, no
 * wait), and is put back if the API refuses.
 */
export function useRouteEnquiry(ref: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: RouteRequest) => enquiriesApi.route(ref, body),
    onMutate: async (body) => {
      await queryClient.cancelQueries({ queryKey: enquiryKeys.lists() });
      const snapshot = queryClient.getQueriesData<EnquiriesResponse>({ queryKey: enquiryKeys.lists() });
      queryClient.setQueriesData<EnquiriesResponse>({ queryKey: enquiryKeys.lists() }, (old) =>
        old ? { enquiries: old.enquiries.map((e) => (e.ref === ref ? { ...e, lane: body.lane } : e)) } : old,
      );
      return { snapshot };
    },
    onError: (_error, _body, context) => {
      context?.snapshot.forEach(([key, data]) => queryClient.setQueryData(key, data));
    },
    onSuccess: (data) => queryClient.setQueryData(enquiryKeys.detail(ref), data),
    onSettled: () => queryClient.invalidateQueries({ queryKey: enquiryKeys.lists() }),
  });
}

export const useCorrectEnquiry = (ref: string) =>
  useEnquiryAction(ref, (body: CorrectEnquiryRequest) => enquiriesApi.correct(ref, body));

export const useEditDraft = (ref: string) => useEnquiryAction(ref, (body: string) => enquiriesApi.editDraft(ref, body));

export const useSendReply = (ref: string) => useEnquiryAction(ref, () => enquiriesApi.send(ref));

export const useSetEnquiryStatus = (ref: string) =>
  useEnquiryAction(ref, (status: EnquiryStatus) => enquiriesApi.setStatus(ref, status));

export const useRetryAi = (ref: string) => useEnquiryAction(ref, () => enquiriesApi.retryAi(ref));
