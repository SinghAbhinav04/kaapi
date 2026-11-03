/** Booked-date windows, in days ending today. */
export const RANGE_OPTIONS = [
  { value: '30', label: '30 days' },
  { value: '90', label: '90 days' },
  { value: '365', label: '12 months' },
] as const;

export type RangeValue = (typeof RANGE_OPTIONS)[number]['value'];
export const DEFAULT_RANGE: RangeValue = '90';

/** How many x-axis date labels the daily chart aims for. */
export const DAILY_LABEL_TARGET = 6;
/** How many gridlines a value axis aims for. */
export const TICK_TARGET = 4;

export const BOOKING_STATUS_LABEL: Record<string, string> = {
  CONFIRMED: 'Confirmed',
  PENDING_PAYMENT: 'Awaiting payment',
  PAYMENT_FAILED: 'Payment failed',
  EXPIRED: 'Expired unpaid',
  CANCELLED: 'Cancelled',
  REFUND_PENDING: 'Refund due',
  REFUNDED: 'Refunded',
};

export const CHANNEL_LABEL: Record<string, string> = {
  WEB_FORM: 'Website form',
  EMAIL: 'Email',
  WHATSAPP: 'WhatsApp',
  EVENT_FORM: 'Event form',
};

export const CATEGORY_LABEL: Record<string, string> = {
  ROOM: 'Room stays',
  WEDDING: 'Weddings',
  OFFSITE: 'Offsites',
  COMPLAINT: 'Complaints',
  OTHER: 'Other',
  UNREAD: 'Not read yet',
};

export const COPY = {
  title: 'Analytics',
  description: 'Direct bookings by the date they were made. Every number comes from bookings and inventory, never from the AI.',
  commissionNote: 'Estimate: about 20% of room revenue, the commission agencies take (from the brief).',
  onTheBooksNote: 'Nights already sold on the website for this month and the next five, against its allotment.',
  revenueTitle: 'Revenue by day booked',
  emptyRange: 'No confirmed bookings in this range.',
} as const;
