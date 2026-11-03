import type { PillTone } from '@/shared/ui';

export const OUTBOX_KIND: Record<string, { label: string; tone: PillTone }> = {
  OTP: { label: 'Sign-in code', tone: 'neutral' },
  BOOKING_CONFIRMATION: { label: 'Booking confirmation', tone: 'estate' },
  ENQUIRY_REPLY: { label: 'Enquiry reply', tone: 'estate' },
  ALERT: { label: 'Staff alert', tone: 'alert' },
  REFUND_NOTICE: { label: 'Refund notice', tone: 'amber' },
  STAFF_MESSAGE: { label: 'New message', tone: 'estate' },
};

/** Whether a message really left the building. */
export const OUTBOX_STATUS: Record<string, { label: string; tone: PillTone }> = {
  SENT: { label: 'Sent via Gmail', tone: 'estate' },
  SIMULATED: { label: 'Simulated', tone: 'neutral' },
  FAILED: { label: 'Failed', tone: 'alert' },
};

export const GMAIL_COPY = {
  notConfigured: 'Gmail is not set up on the server, so replies are simulated. Add GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET to connect a mailbox.',
  disconnected: 'No mailbox connected: replies are simulated in the Outbox.',
  disconnectedStaff: 'No mailbox connected yet. Ask the GM or admin to connect Gmail; until then replies are simulated.',
  connected: (email: string) => `Reading and replying as ${email}`,
  returned: {
    connected: 'Gmail connected. New mail will appear in the inbox within a minute.',
    cancelled: 'Gmail connection cancelled.',
    error: 'Gmail could not be connected.',
  } as Record<string, string>,
} as const;

export const LANE_NAME: Record<string, string> = {
  RESERVATIONS: 'Reservations',
  SALES: 'Sales',
  GM: 'General Manager',
  UNSORTED: 'Unsorted',
};
