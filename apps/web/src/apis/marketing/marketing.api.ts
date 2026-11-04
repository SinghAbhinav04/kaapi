import { http, type HttpClient } from '@/apis/http';

import { MARKETING_PATHS } from './const';
import type { MarketingApi } from './interface';

export function createMarketingApi(client: HttpClient): MarketingApi {
  return {
    unsubscribe: (token) => client.post(MARKETING_PATHS.unsubscribe, { token }),
  };
}

export const marketingApi = createMarketingApi(http);
