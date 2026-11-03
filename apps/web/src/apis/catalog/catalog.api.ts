import { http, type HttpClient } from '@/apis/http';

import { CATALOG_PATHS } from './const';
import type { CatalogApi } from './interface';

export function createCatalogApi(client: HttpClient): CatalogApi {
  return {
    getProperty: (signal) => client.get(CATALOG_PATHS.property, { signal }),
    listRooms: (signal) => client.get(CATALOG_PATHS.rooms, { signal }),
    getRoom: (slug, signal) => client.get(CATALOG_PATHS.room(slug), { signal }),
    getFxRates: (signal) => client.get(CATALOG_PATHS.fxRates, { signal }),
    getCalendar: (slug, params, signal) =>
      client.get(CATALOG_PATHS.calendar(slug), { signal, query: { from: params.from, to: params.to, plan: params.plan } }),
    searchAvailability: (params, signal) =>
      client.get(CATALOG_PATHS.availability, {
        signal,
        query: {
          check_in: params.checkIn,
          check_out: params.checkOut,
          adults: params.adults,
          children: params.children,
          rooms: params.rooms,
          promo: params.promo,
        },
      }),
  };
}

export const catalogApi = createCatalogApi(http);
