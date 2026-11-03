export const STATUS_FILTERS = [
  { value: '', label: 'All' },
  { value: 'CONFIRMED', label: 'Confirmed' },
  { value: 'PENDING_PAYMENT', label: 'Awaiting payment' },
  { value: 'REFUND_PENDING', label: 'Refund due' },
  { value: 'CANCELLED', label: 'Cancelled' },
  { value: 'EXPIRED', label: 'Expired' },
] as const;

export const BOOKINGS_LIMIT = 200;
