/** Payment statuses after which the guest belongs on the confirmation page. */
export const PAID_STATUSES: ReadonlySet<string> = new Set(['CAPTURED', 'REFUND_PENDING', 'REFUNDED']);

export const METHODS = [
  { value: 'UPI', label: 'UPI' },
  { value: 'CARD', label: 'Card' },
  { value: 'NETBANKING', label: 'Net banking' },
  { value: 'WALLET', label: 'Wallet' },
] as const;
export type Method = (typeof METHODS)[number]['value'];

/** A placeholder merchant: the QR is a real UPI request, but this VPA receives nothing. */
export const UPI_PAYEE = { vpa: 'kaapiestate@okaxis', name: 'Kaapi Estate' } as const;

/** Test mode: how long a shown QR waits before the "scan" succeeds. */
export const QR_AUTO_PAY_SECONDS = 8;

export const BANKS = [
  { value: 'SBI', label: 'State Bank of India' },
  { value: 'HDFC', label: 'HDFC Bank' },
  { value: 'ICICI', label: 'ICICI Bank' },
  { value: 'AXIS', label: 'Axis Bank' },
  { value: 'KOTAK', label: 'Kotak Mahindra Bank' },
] as const;

export const WALLETS = [
  { value: 'PAYTM', label: 'Paytm' },
  { value: 'PHONEPE', label: 'PhonePe' },
  { value: 'AMAZONPAY', label: 'Amazon Pay' },
] as const;
