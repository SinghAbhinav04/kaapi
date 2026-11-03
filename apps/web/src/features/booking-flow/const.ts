/** sessionStorage keys: per tab, gone when the tab closes. */
export const STORAGE_KEY = {
  hold: (holdId: string) => `kaapi.hold.${holdId}`,
  booking: (ref: string) => `kaapi.booking.${ref}`,
} as const;

/** The booking flow is three steps at most (brief requirement). */
export const BOOKING_STEPS = ['Room & dates', 'Your details', 'Payment'] as const;

export const CANCELLATION_DAYS = 7;
