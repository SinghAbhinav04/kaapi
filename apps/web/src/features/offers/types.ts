import type { OfferRequest } from '@/apis/offers';

/** What the dialog edits: amounts as people type them (15 for 15%, 500 for ₹500). */
export interface OfferForm {
  code: string;
  description: string;
  type: OfferRequest['type'];
  amount: string;
  stayFrom: string;
  stayTo: string;
  bookFrom: string;
  bookTo: string;
  minNights: string;
  maxUses: string;
  roomTypeIds: string[];
  autoApply: boolean;
  active: boolean;
}

export type OfferState = 'LIVE' | 'SCHEDULED' | 'ENDED' | 'PAUSED' | 'USED_UP';
