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
  dashboard: () =>
    queryOptions({
      queryKey: operationsKeys.dashboard(),
      queryFn: ({ signal }) => operationsApi.dashboard(signal),
      refetchInterval: OPERATIONS_REFRESH_MS,
    }),
};
