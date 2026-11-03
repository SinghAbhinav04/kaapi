import { keepPreviousData, queryOptions } from '@tanstack/react-query';

import { catalogApi } from './catalog.api';
import { CATALOG_STALE_MS, catalogKeys } from './const';
import type { AvailabilityParams, CalendarParams } from './types';

/**
 * Query definitions shared by server prefetching and client hooks, so both use the same
 * key and fetcher and the server's data hydrates straight into the client cache.
 */
export const catalogQueries = {
  property: () =>
    queryOptions({
      queryKey: catalogKeys.property(),
      queryFn: ({ signal }) => catalogApi.getProperty(signal),
      staleTime: CATALOG_STALE_MS.property,
    }),
  rooms: () =>
    queryOptions({
      queryKey: catalogKeys.rooms(),
      queryFn: ({ signal }) => catalogApi.listRooms(signal),
      staleTime: CATALOG_STALE_MS.rooms,
    }),
  room: (slug: string) =>
    queryOptions({
      queryKey: catalogKeys.room(slug),
      queryFn: ({ signal }) => catalogApi.getRoom(slug, signal),
      staleTime: CATALOG_STALE_MS.rooms,
    }),
  // Month to month and date to date, the previous result stays visible while the next loads.
  calendar: (slug: string, params: CalendarParams) =>
    queryOptions({
      queryKey: catalogKeys.calendar(slug, params),
      queryFn: ({ signal }) => catalogApi.getCalendar(slug, params, signal),
      staleTime: CATALOG_STALE_MS.calendar,
      placeholderData: keepPreviousData,
    }),
  availability: (params: AvailabilityParams) =>
    queryOptions({
      queryKey: catalogKeys.availability(params),
      queryFn: ({ signal }) => catalogApi.searchAvailability(params, signal),
      staleTime: CATALOG_STALE_MS.availability,
      placeholderData: keepPreviousData,
    }),
};
