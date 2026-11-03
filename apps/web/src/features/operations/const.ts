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
