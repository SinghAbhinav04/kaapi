export const OPERATIONS_PATHS = {
  outbox: '/v1/staff/outbox',
  dashboard: '/v1/staff/dashboard',
} as const;

export const operationsKeys = {
  all: ['operations'] as const,
  outbox: (limit: number) => [...operationsKeys.all, 'outbox', limit] as const,
  dashboard: () => [...operationsKeys.all, 'dashboard'] as const,
};

export const OUTBOX_LIMIT = 100;
export const OPERATIONS_REFRESH_MS = 30_000;
