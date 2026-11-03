export const OPERATIONS_PATHS = {
  outbox: '/v1/staff/outbox',
  dashboard: '/v1/staff/dashboard',
  messages: '/v1/staff/messages',
  gmail: '/v1/staff/integrations/gmail',
  gmailConnect: '/v1/staff/integrations/gmail/connect',
  gmailSync: '/v1/staff/integrations/gmail/sync',
} as const;

export const operationsKeys = {
  all: ['operations'] as const,
  outbox: (limit: number) => [...operationsKeys.all, 'outbox', limit] as const,
  outboxes: () => [...operationsKeys.all, 'outbox'] as const,
  dashboard: () => [...operationsKeys.all, 'dashboard'] as const,
  gmail: () => [...operationsKeys.all, 'gmail'] as const,
};

export const OUTBOX_LIMIT = 100;
export const OPERATIONS_REFRESH_MS = 30_000;
