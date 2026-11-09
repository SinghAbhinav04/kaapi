import type { Category, Channel, Lane } from '@/apis/enquiries';
import type { PillTone } from '@/shared/ui';

export const CHANNEL_LABEL: Record<Channel, string> = {
  WEB_FORM: 'Website',
  EMAIL: 'Email',
  WHATSAPP: 'WhatsApp',
  EVENT_FORM: 'Event form',
};

/** The Send button names the channel the reply will go out on. */
export const SEND_LABEL: Record<Channel, string> = {
  WEB_FORM: 'Send by email',
  EMAIL: 'Send by email',
  WHATSAPP: 'Send on WhatsApp',
  EVENT_FORM: 'Send by email',
};

export const CATEGORY_LABEL: Record<Category, string> = {
  ROOM: 'Room stay',
  WEDDING: 'Wedding',
  OFFSITE: 'Offsite / group',
  COMPLAINT: 'Complaint',
  OTHER: 'Other',
};

export const CATEGORY_TONE: Record<Category, PillTone> = {
  ROOM: 'estate',
  WEDDING: 'amber',
  OFFSITE: 'amber',
  COMPLAINT: 'alert',
  OTHER: 'neutral',
};

export const LANE_LABEL: Record<Lane, string> = {
  RESERVATIONS: 'Reservations',
  SALES: 'Sales',
  GM: 'General Manager',
  UNSORTED: 'Unsorted',
};

/** The lane each category belongs to, for the one-click re-route menu. */
export const CATEGORY_LANE: Record<Category, Lane> = {
  ROOM: 'RESERVATIONS',
  WEDDING: 'SALES',
  OFFSITE: 'SALES',
  COMPLAINT: 'GM',
  OTHER: 'UNSORTED',
};

export const INBOX_LIMIT = 300;

/** Above a later message from the guest in the same email thread. */
export const FOLLOW_UP_LABEL = 'The guest replied';

/** Replies are signed by whoever sends them; the API adds the sign-off at send time. */
export const SIGN_OFF_COPY = {
  unsent: 'Your sign-off, added when you send',
  sent: 'Signed by the sender',
  editHint: "Don't sign it: your name and team are added below when you send.",
} as const;

/** The server writes quote lines like "- Coffee Villa: ₹44,368 room only, ₹47,200 with breakfast (1 left)". */
export const QUOTE_LINE = /^- (.+?): (₹[\d,]+) room only(?:, (₹[\d,]+) with breakfast)?(?: \((\d+) left\))?$/;
export const QUOTE_HEAD = /^For .+totals including GST:$/;
