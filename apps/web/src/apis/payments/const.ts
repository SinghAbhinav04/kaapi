export const PAYMENT_PATHS = {
  page: (orderId: string) => `/v1/payments/${encodeURIComponent(orderId)}`,
  simulate: (orderId: string) => `/v1/payments/${encodeURIComponent(orderId)}/simulate`,
} as const;

export const paymentKeys = {
  all: ['payments'] as const,
  page: (orderId: string) => [...paymentKeys.all, orderId] as const,
};
