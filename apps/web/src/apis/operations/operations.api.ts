import { http, type HttpClient } from '@/apis/http';

import { OPERATIONS_PATHS } from './const';
import type { OperationsApi } from './interface';

export function createOperationsApi(client: HttpClient): OperationsApi {
  return {
    outbox: (limit, signal) => client.get(OPERATIONS_PATHS.outbox, { signal, query: { limit } }),
    dashboard: (signal) => client.get(OPERATIONS_PATHS.dashboard, { signal }),
    compose: (body) => client.post(OPERATIONS_PATHS.messages, body),
    gmailStatus: (signal) => client.get(OPERATIONS_PATHS.gmail, { signal }),
    gmailConnect: () => client.post(OPERATIONS_PATHS.gmailConnect),
    gmailSync: () => client.post(OPERATIONS_PATHS.gmailSync),
    gmailDisconnect: () => client.delete(OPERATIONS_PATHS.gmail),
  };
}

export const operationsApi = createOperationsApi(http);
