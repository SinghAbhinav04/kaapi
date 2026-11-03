'use client';

import { useQuery } from '@tanstack/react-query';

import { catalogQueries } from './queries';
import type { AvailabilityParams, CalendarParams } from './types';

export const useProperty = () => useQuery(catalogQueries.property());

export const useRooms = () => useQuery(catalogQueries.rooms());

export const useRoom = (slug: string) => useQuery(catalogQueries.room(slug));

/** Pass false to skip the request (e.g. while prices are shown in INR only). */
export const useFxRates = (enabled = true) => useQuery({ ...catalogQueries.fxRates(), enabled });

export const useCalendar = (slug: string, params: CalendarParams) => useQuery(catalogQueries.calendar(slug, params));

/** Pass null to wait until the guest has picked dates. */
export const useAvailability = (params: AvailabilityParams | null) =>
  useQuery({
    ...catalogQueries.availability(params ?? EMPTY_PARAMS),
    enabled: params !== null,
  });

const EMPTY_PARAMS: AvailabilityParams = { checkIn: '', checkOut: '', adults: 2, children: 0, rooms: 1 };
