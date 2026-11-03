import type { Hold } from '@/apis/booking';
import type { Photo } from '@/apis/catalog';

export type BookingStep = 1 | 2 | 3;

/**
 * What this tab knows about its hold. The token proves this browser created the hold; it
 * is the only key to turn it into a booking or to read the booking back before sign-in.
 */
export interface HoldTicket {
  holdId: string;
  token: string;
  roomSlug: string;
  roomName: string;
  planName: string;
  refundable: boolean;
  adults: number;
  children: number;
  hold: Hold;
  photo?: Photo;
}
