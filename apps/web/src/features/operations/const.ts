import type { OutboxMessage } from '@/apis/operations';
import type { PillTone } from '@/shared/ui';

export const OUTBOX_KIND: Record<OutboxMessage['kind'], { label: string; tone: PillTone }> = {
  OTP: { label: 'Sign-in code', tone: 'neutral' },
  WELCOME: { label: 'Welcome', tone: 'neutral' },
  BOOKING_CONFIRMATION: { label: 'Booking confirmation', tone: 'estate' },
  ENQUIRY_REPLY: { label: 'Enquiry reply', tone: 'estate' },
  ALERT: { label: 'Staff alert', tone: 'alert' },
  REFUND_NOTICE: { label: 'Refund notice', tone: 'amber' },
  STAFF_MESSAGE: { label: 'New message', tone: 'estate' },
  STAFF_CODE: { label: 'Staff sign-in code', tone: 'neutral' },
  CHECKIN_REMINDER: { label: 'Arrival reminder', tone: 'estate' },
  CHECKOUT_REMINDER: { label: 'Check-out reminder', tone: 'neutral' },
  PROMOTION: { label: 'Offer email', tone: 'estate' },
};

/** Whether a message really left the building. */
export const OUTBOX_STATUS: Record<OutboxMessage['status'], { label: string; tone: PillTone }> = {
  QUEUED: { label: 'Sending…', tone: 'amber' },
  SENT: { label: 'Sent via Gmail', tone: 'estate' },
  SIMULATED: { label: 'Simulated', tone: 'neutral' },
  FAILED: { label: 'Failed', tone: 'alert' },
};

export const LANE_NAME: Record<string, string> = {
  RESERVATIONS: 'Reservations',
  SALES: 'Sales',
  GM: 'General Manager',
  UNSORTED: 'Unsorted',
};

/** The Today page looks this many days ahead (the API sends the same window). */
export const WEEK_DAYS = 7;

/** The Today page: the desk's sheet for the day and the week ahead. */
export const TODAY_COPY = {
  title: 'Today',
  description: (day: string) => `${day}. What needs a reply now, who arrives and leaves this week, and how the week is going.`,
  owed: 'Owed a first reply',
  owedSub: (target: number) => `Open, with no reply after ${target} min`,
  complaints: 'Open complaints',
  complaintsSub: 'The GM was alerted on each',
  refunds: 'Refunds to make',
  refundsSub: 'Payments that arrived after the room went',
  tonight: 'Sold tonight',
  tonightSub: (allotment: number) => `of ${allotment} website rooms`,
  owedTitle: 'Waiting for a reply',
  owedMore: (n: number) => `and ${n} more in the inbox`,
  owedNone: 'Every open enquiry has had a reply.',
  urgent: 'Urgent',
  waiting: (age: string) => (/\d (min|h)/.test(age) ? `waiting ${age}` : `since ${age}`),
  weekTitle: 'Arrivals and departures, next 7 days',
  weekNone: 'No direct-booking arrivals or departures this week.',
  todayLabel: 'Today',
  arriving: 'Arriving',
  leaving: 'Leaving',
  goalsTitle: 'This week against the goals',
  medianReply: 'Median first reply',
  medianReplySub: (target: number, replied: number) => `Target ${target} min · ${replied} replied`,
  direct: 'Direct bookings',
  directSub: (revenue: string) => `${revenue} with no agency commission`,
  overbookings: 'Overbookings',
  overbookingsSub: 'The database refuses to sell a room twice',
  aiCorrected: 'AI routing corrected',
  aiWaiting: (n: number) => `${n} waiting for the AI`,
  aiOff: 'AI is off: keyword rules only',
} as const;
