import type { PillTone } from '@/shared/ui';

import type { OfferForm, OfferState } from './types';

export const COPY = {
  title: 'Offers',
  description:
    'Festive and seasonal discounts that apply by themselves, and promo codes guests type. Guests always get their best offer; prices still come from the rate table.',
  add: 'New offer',
  empty: 'No offers yet. Add a festive or seasonal discount to start.',
  autoHint: 'Applies by itself to stays it covers, booked while it runs. Off: guests must type the code.',
  roomsHint: 'Leave all unticked for every room type.',
  usesHint: 'Leave empty for no limit.',
} as const;

export const TYPE_OPTIONS = [
  { value: 'PERCENT', label: '% off' },
  { value: 'FLAT', label: '₹ off' },
] as const;

export const STATE_PILL: Record<OfferState, { label: string; tone: PillTone }> = {
  LIVE: { label: 'Live', tone: 'estate' },
  SCHEDULED: { label: 'Scheduled', tone: 'amber' },
  ENDED: { label: 'Ended', tone: 'neutral' },
  PAUSED: { label: 'Paused', tone: 'neutral' },
  USED_UP: { label: 'Used up', tone: 'neutral' },
};

export const EMPTY_FORM: OfferForm = {
  code: '',
  description: '',
  type: 'PERCENT',
  amount: '',
  stayFrom: '',
  stayTo: '',
  bookFrom: '',
  bookTo: '',
  minNights: '1',
  maxUses: '',
  roomTypeIds: [],
  autoApply: true,
  active: true,
};
