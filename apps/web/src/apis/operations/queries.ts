import { queryOptions } from '@tanstack/react-query';

import { OPERATIONS_REFRESH_MS, OUTBOX_LIMIT, operationsKeys } from './const';
import { operationsApi } from './operations.api';

export const operationsQueries = {
  outbox: (limit = OUTBOX_LIMIT) =>
    queryOptions({
      queryKey: operationsKeys.outbox(limit),
      queryFn: ({ signal }) => operationsApi.outbox(limit, signal),
      refetchInterval: OPERATIONS_REFRESH_MS,
    }),
  // Bookings, inventory and enquiries changes refetch it through the change stream (apis/changes).
  dashboard: () =>
    queryOptions({
      queryKey: operationsKeys.dashboard(),
      queryFn: ({ signal }) => operationsApi.dashboard(signal),
      staleTime: Infinity,
    }),
  /** Refreshes on the inbox's rhythm so "last synced" stays honest. */
  gmail: () =>
    queryOptions({
      queryKey: operationsKeys.gmail(),
      queryFn: ({ signal }) => operationsApi.gmailStatus(signal),
      refetchInterval: OPERATIONS_REFRESH_MS,
    }),
};
