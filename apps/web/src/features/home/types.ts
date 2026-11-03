import type { AvailabilityOption, Photo } from '@/apis/catalog';

/** One stay card: the room, plus what it costs and whether it's free for the searched dates. */
export interface StayCardModel {
  slug: string;
  name: string;
  facts: string;
  photo: Photo | undefined;
  status: AvailabilityOption['status'] | 'UNKNOWN';
  /** The flexible room-only total for the stay, GST included. */
  fromPaise: number | null;
  left: number | null;
  /** Why it can't be booked, in the guest's words. */
  note: string | null;
}
