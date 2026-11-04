import type { Offer, OfferRequest } from '@/apis/offers';
import { formatINR, paiseToRupees, rupeesToPaise } from '@/shared/lib/money';

import { EMPTY_FORM } from './const';
import type { OfferForm, OfferState } from './types';

/** Basis points per percent: the API stores 15% as 1500. */
const BPS_PER_PERCENT = 100;

export function toForm(o?: Offer): OfferForm {
  if (!o) return EMPTY_FORM;
  return {
    code: o.code,
    description: o.description,
    type: o.type,
    amount: String(o.type === 'PERCENT' ? o.value / BPS_PER_PERCENT : paiseToRupees(o.value)),
    stayFrom: o.stay_from,
    stayTo: o.stay_to,
    bookFrom: o.book_from,
    bookTo: o.book_to,
    minNights: String(o.min_nights),
    maxUses: o.max_uses === null ? '' : String(o.max_uses),
    roomTypeIds: o.room_type_ids,
    autoApply: o.auto_apply,
    active: o.active,
    announce: false,
  };
}

export function toRequest(f: OfferForm): OfferRequest {
  const amount = Number(f.amount);
  return {
    code: f.code.trim().toUpperCase(),
    description: f.description.trim(),
    type: f.type,
    value: f.type === 'PERCENT' ? Math.round(amount * BPS_PER_PERCENT) : rupeesToPaise(amount),
    stay_from: f.stayFrom,
    stay_to: f.stayTo,
    book_from: f.bookFrom,
    book_to: f.bookTo,
    min_nights: Number(f.minNights) || 1,
    max_uses: f.maxUses.trim() === '' ? null : Number(f.maxUses),
    room_type_ids: f.roomTypeIds,
    auto_apply: f.autoApply,
    active: f.active,
    announce: f.announce,
  };
}

/** "15% off" or "₹500 off". */
export function describeValue(o: Pick<Offer, 'type' | 'value'>): string {
  return o.type === 'PERCENT' ? `${o.value / BPS_PER_PERCENT}% off` : `${formatINR(o.value)} off`;
}

/** Where an offer stands today (YYYY-MM-DD, the resort's date). */
export function offerState(o: Offer, today: string): OfferState {
  if (!o.active) return 'PAUSED';
  if (o.max_uses !== null && o.used_count >= o.max_uses) return 'USED_UP';
  if (today < o.book_from) return 'SCHEDULED';
  if (today > o.book_to) return 'ENDED';
  return 'LIVE';
}
