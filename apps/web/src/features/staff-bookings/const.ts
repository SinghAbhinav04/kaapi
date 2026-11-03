export const STATUS_FILTERS = [
  { value: '', label: 'All' },
  { value: 'CONFIRMED', label: 'Confirmed' },
  { value: 'PENDING_PAYMENT', label: 'Awaiting payment' },
  { value: 'REFUND_PENDING', label: 'Refund due' },
  { value: 'CANCELLED', label: 'Cancelled' },
  { value: 'EXPIRED', label: 'Expired' },
] as const;

export const BOOKINGS_LIMIT = 200;

/** Live bookings, or the archive the retention job moves finished stays into. */
export const VIEW_FILTERS = [
  { value: 'live', label: 'Current' },
  { value: 'archived', label: 'Archived' },
] as const;

export const ARCHIVE_NOTE =
  'Stays that are over. Kept for 8 financial years for tax; guest contact details are removed 12 months after check-out.';
