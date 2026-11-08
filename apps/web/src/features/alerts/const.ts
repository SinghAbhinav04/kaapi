import type { Notification } from '@/apis/notifications';

/** How each kind of alert is labelled in the list. */
export const ALERT_LABEL: Record<Notification['type'], string> = {
  COMPLAINT_ALERT: 'Complaint',
  UNSORTED: 'Needs sorting',
  BOOKING: 'Booking',
  REFUND_NEEDED: 'Refund needed',
  GUEST_REPLY: 'Guest replied',
};

export const ALERTS_COPY = {
  button: 'Alerts',
  title: 'Alerts',
  empty: 'Nothing needs you right now.',
  markAll: 'Mark all read',
} as const;
