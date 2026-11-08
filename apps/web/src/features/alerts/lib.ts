import { ROUTES } from '@/shared/const';

import type { Notification } from '@/apis/notifications';

/** Where an alert takes you: the enquiry it's about, else the bookings list. */
export function alertHref(n: Notification): string {
  if (n.enquiry_ref) return ROUTES.staff.enquiry(n.enquiry_ref);
  if (n.booking_ref) return ROUTES.staff.bookings;
  return ROUTES.staff.inbox;
}
