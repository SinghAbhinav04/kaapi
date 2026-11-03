import type { PillTone } from '@/shared/ui';

export const OUTBOX_KIND: Record<string, { label: string; tone: PillTone }> = {
  OTP: { label: 'Sign-in code', tone: 'neutral' },
  BOOKING_CONFIRMATION: { label: 'Booking confirmation', tone: 'estate' },
  ENQUIRY_REPLY: { label: 'Enquiry reply', tone: 'estate' },
  ALERT: { label: 'Staff alert', tone: 'alert' },
  REFUND_NOTICE: { label: 'Refund notice', tone: 'amber' },
};

export const LANE_NAME: Record<string, string> = {
  RESERVATIONS: 'Reservations',
  SALES: 'Sales',
  GM: 'General Manager',
  UNSORTED: 'Unsorted',
};
