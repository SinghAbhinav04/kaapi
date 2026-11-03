'use client';

import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import { operationsKeys } from './const';
import { operationsApi } from './operations.api';
import { operationsQueries } from './queries';
import type { ComposeRequest } from './types';

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
