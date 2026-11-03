export const CHANGES_PATHS = { stream: '/v1/staff/changes/stream' } as const;

/** Topics the API sends (see migrations/0013_data_changes.sql); "*" means anything may have changed. */
export const CHANGE_TOPICS = ['bookings', 'inventory', 'offers', 'enquiries', '*'] as const;

/** After the stream closes for good (e.g. the session ended), how long before reopening it. */
export const CHANGES_RETRY_MS = 5_000;
