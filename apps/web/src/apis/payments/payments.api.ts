import { http, type HttpClient } from '@/apis/http';

import { PAYMENT_PATHS } from './const';
import type { PaymentsApi } from './interface';

export function createPaymentsApi(client: HttpClient): PaymentsApi {
  return {
    getPage: (orderId, signal) => client.get(PAYMENT_PATHS.page(orderId), { signal }),
    simulate: (orderId, outcome) => client.post(PAYMENT_PATHS.simulate(orderId), { outcome }),
  };
}

export const paymentsApi = createPaymentsApi(http);
