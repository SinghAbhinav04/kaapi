import { HydrationBoundary, dehydrate, type QueryClient } from '@tanstack/react-query';
import type { ReactNode } from 'react';

/**
 * Hands what a server component prefetched to the client cache, so the client renders the
 * same data immediately and only refetches once it goes stale.
 */
export function Hydrate({ client, children }: { client: QueryClient; children: ReactNode }) {
  return <HydrationBoundary state={dehydrate(client)}>{children}</HydrationBoundary>;
}
