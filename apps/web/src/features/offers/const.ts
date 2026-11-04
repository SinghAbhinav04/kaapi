import type { Campaign } from '@/apis/offers';
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
  announce: (n: number) => `Email it to the ${n} ${n === 1 ? 'guest' : 'guests'} who asked for offers. Sent once; later edits don't email again.`,
  campaignsTitle: 'Offer emails',
  campaignsLead: (n: number) =>
    `Go only to guests who ticked "send me offers" at checkout: ${n} right now. Each guest gets each email once, with a one-click unsubscribe.`,
  campaignsEmpty: 'No offer emails yet. Tick "Email it" on an offer, or write a seasonal note below.',
  campaignSent: 'Scheduled. It goes out within a minute.',
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
  announce: false,
};

export const CAMPAIGN_KINDS = [
  { value: 'FESTIVE', label: 'Festive or seasonal' },
  { value: 'ANNOUNCEMENT', label: 'Announcement' },
] as const;

export const CAMPAIGN_KIND_LABEL: Record<Campaign['kind'], string> = { OFFER: 'Offer', FESTIVE: 'Festive', ANNOUNCEMENT: 'Announcement' };

export const CAMPAIGN_STATUS: Record<Campaign['status'], { label: string; tone: PillTone }> = {
  SCHEDULED: { label: 'Scheduled', tone: 'amber' },
  SENDING: { label: 'Sending', tone: 'amber' },
  SENT: { label: 'Sent', tone: 'estate' },
  CANCELLED: { label: 'Cancelled', tone: 'neutral' },
};
