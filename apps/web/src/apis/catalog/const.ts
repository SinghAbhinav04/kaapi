import type { AvailabilityParams, CalendarParams } from './types';

export const CATALOG_PATHS = {
  property: '/v1/property',
  rooms: '/v1/rooms',
  room: (slug: string) => `/v1/rooms/${encodeURIComponent(slug)}`,
  calendar: (slug: string) => `/v1/rooms/${encodeURIComponent(slug)}/calendar`,
  availability: '/v1/availability',
  fxRates: '/v1/fx-rates',
} as const;

/** Query keys, hierarchical so a whole branch can be invalidated at once. */
export const catalogKeys = {
  all: ['catalog'] as const,
  property: () => [...catalogKeys.all, 'property'] as const,
  rooms: () => [...catalogKeys.all, 'rooms'] as const,
  room: (slug: string) => [...catalogKeys.rooms(), slug] as const,
  calendar: (slug: string, params: CalendarParams) => [...catalogKeys.room(slug), 'calendar', params] as const,
  availability: (params: AvailabilityParams) => [...catalogKeys.all, 'availability', params] as const,
  fxRates: () => [...catalogKeys.all, 'fx-rates'] as const,
};

/**
 * How long data counts as fresh. Room descriptions barely change; prices and rooms left
 * do, so they refresh quickly, but stale values stay on screen while they do.
 */
export const CATALOG_STALE_MS = {
  property: 60 * 60_000,
  rooms: 10 * 60_000,
  calendar: 60_000,
  availability: 30_000,
  // The ECB publishes once a working day; an hour-old estimate is as good as a fresh one.
  fxRates: 60 * 60_000,
} as const;
