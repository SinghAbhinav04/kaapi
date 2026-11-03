import { http, type HttpClient } from '@/apis/http';

import { OPERATIONS_PATHS } from './const';
import type { OperationsApi } from './interface';

export function createOperationsApi(client: HttpClient): OperationsApi {
  return {
    outbox: (limit, signal) => client.get(OPERATIONS_PATHS.outbox, { signal, query: { limit } }),
    dashboard: (signal) => client.get(OPERATIONS_PATHS.dashboard, { signal }),
  };
}

export const operationsApi = createOperationsApi(http);
