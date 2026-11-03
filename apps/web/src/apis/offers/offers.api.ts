import { http, type HttpClient } from '@/apis/http';

import { OFFERS_PATHS } from './const';
import type { OffersApi } from './interface';

export function createOffersApi(client: HttpClient): OffersApi {
  return {
    list: (signal) => client.get(OFFERS_PATHS.offers, { signal }),
    create: (body) => client.post(OFFERS_PATHS.offers, body),
    update: (id, body) => client.put(OFFERS_PATHS.offer(id), body),
  };
}

export const offersApi = createOffersApi(http);
