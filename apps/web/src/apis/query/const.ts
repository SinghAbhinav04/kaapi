/** Defaults for every query unless a module's queryOptions says otherwise. */
export const QUERY_DEFAULTS = {
  staleTime: 30_000,
  /** Unused data stays cached this long, so going back to a page is instant. */
  gcTime: 10 * 60_000,
  maxRetries: 2,
} as const;
