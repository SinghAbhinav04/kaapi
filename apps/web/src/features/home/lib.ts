import type { AvailabilityResponse, Room } from '@/apis/catalog';
import { plural } from '@/shared/lib/text';

import { HEADLINE_PLAN } from './const';
import type { StayCardModel } from './types';

/** "85 SQM · KING + DAY BED · SLEEPS 4 + 2" */
export function roomFacts(room: Room): string {
  return [`${room.size_sqm} SQM`, room.bed.toUpperCase(), `SLEEPS ${room.sleeps.adults} + ${room.sleeps.children}`].join(' · ');
}

const NOTES: Record<string, (minStay: number) => string> = {
  SOLD_OUT: () => 'Sold out online for these dates',
  RESTRICTED: (minStay) => (minStay > 1 ? `${plural(minStay, 'night')} minimum on these dates` : 'Not available to start on these dates'),
  TOO_MANY_GUESTS: () => 'Too many guests for one of these',
  NOT_ON_SALE: () => 'Not on sale online for these dates',
};

/** Merges the room list with the availability search into card data. Pure. */
export function toStayCards(rooms: readonly Room[], availability: AvailabilityResponse | undefined): StayCardModel[] {
  const bySlug = new Map(availability?.options.map((o) => [o.slug, o]));
  return rooms.map((room) => {
    const option = bySlug.get(room.slug);
    // The headline price is the flexible room-only rate, as on the design: the cheapest plan is
    // often non-refundable, and leading with it would oversell what the guest gets.
    const headline = option?.plans.find((p) => p.code === HEADLINE_PLAN) ?? option?.plans[0];
    return {
      slug: room.slug,
      name: room.name,
      facts: roomFacts(room),
      photo: room.photos[0],
      status: option?.status ?? 'UNKNOWN',
      fromPaise: option?.status === 'AVAILABLE' ? (headline?.quote.total_paise ?? null) : null,
      left: option?.status === 'AVAILABLE' ? option.left : null,
      note: option && option.status !== 'AVAILABLE' ? (NOTES[option.status]?.(option.min_stay) ?? null) : null,
    };
  });
}
