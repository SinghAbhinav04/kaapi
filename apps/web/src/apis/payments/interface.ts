import type { PaymentOutcome, PaymentPage } from './types';

/** The mock payment gateway. In production this is the provider's hosted checkout. */
export interface PaymentsApi {
  getPage(orderId: string, signal?: AbortSignal): Promise<PaymentPage>;
  /** Signs a webhook event for the outcome and delivers it, as a real gateway would. */
  simulate(orderId: string, outcome: PaymentOutcome): Promise<unknown>;
}
