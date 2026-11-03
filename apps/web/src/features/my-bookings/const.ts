import type { BookingStatus } from '@/apis/booking';
import type { PillTone } from '@/shared/ui';

export const STATUS_LABEL: Record<BookingStatus, { label: string; tone: PillTone }> = {
  CONFIRMED: { label: 'Confirmed', tone: 'estate' },
  PENDING_PAYMENT: { label: 'Awaiting payment', tone: 'amber' },
  PAYMENT_FAILED: { label: 'Payment failed', tone: 'cherry' },
  EXPIRED: { label: 'Expired', tone: 'neutral' },
  CANCELLED: { label: 'Cancelled', tone: 'neutral' },
  REFUND_PENDING: { label: 'Refund on its way', tone: 'amber' },
  REFUNDED: { label: 'Refunded', tone: 'neutral' },
};

export const OTP_LENGTH = 6;
