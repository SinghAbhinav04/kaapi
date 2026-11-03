'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { enquiryKeys } from '@/apis/enquiries';

import { operationsKeys } from './const';
import { operationsApi } from './operations.api';
import { operationsQueries } from './queries';
import type { ComposeRequest, GmailStatusResponse } from './types';

export const useOutbox = (limit?: number) => useQuery(operationsQueries.outbox(limit));

export const useDashboard = (enabled = true) => useQuery({ ...operationsQueries.dashboard(), enabled });

export const useGmailStatus = () => useQuery(operationsQueries.gmail());

export function useCompose() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: ComposeRequest) => operationsApi.compose(body),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: operationsKeys.outboxes() }),
  });
}

/** Sends the browser to Google's consent screen; it comes back to the inbox. */
export const useConnectGmail = () =>
  useMutation({
    mutationFn: () => operationsApi.gmailConnect(),
    onSuccess: ({ auth_url }) => window.location.assign(auth_url),
  });

export function useSyncGmail() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => operationsApi.gmailSync(),
    onSuccess: (res) => {
      queryClient.setQueryData<GmailStatusResponse>(operationsKeys.gmail(), { gmail: res.gmail });
      if (res.imported > 0) void queryClient.invalidateQueries({ queryKey: enquiryKeys.lists() });
    },
  });
}

export function useDisconnectGmail() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => operationsApi.gmailDisconnect(),
    onSuccess: (res) => queryClient.setQueryData(operationsKeys.gmail(), res),
  });
}
