/** Payment statuses after which the guest belongs on the confirmation page. */
export const PAID_STATUSES: ReadonlySet<string> = new Set(['CAPTURED', 'REFUND_PENDING', 'REFUNDED']);
