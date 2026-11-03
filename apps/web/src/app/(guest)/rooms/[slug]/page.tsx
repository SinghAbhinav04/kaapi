import type { Metadata } from 'next';
import { notFound } from 'next/navigation';

import { catalogQueries } from '@/apis/catalog';
import { isApiError } from '@/apis/http';
import { Hydrate, makeQueryClient } from '@/apis/query';
import { CALENDAR_PLAN, RoomPage } from '@/features/room';
import { parseSearch, toAvailabilityParams } from '@/features/stay-search';
import { addMonths, firstOfMonth } from '@/shared/lib/dates';

interface Props {
  params: Promise<{ slug: string }>;
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}

export async function generateMetadata({ params }: Props): Promise<Metadata> {
  const { slug } = await params;
  try {
    const { room } = await makeQueryClient().fetchQuery(catalogQueries.room(slug));
    return { title: room.name, description: room.summary };
  } catch {
    return { title: 'Stay' };
  }
}

/** Server-rendered with the room, this month's prices and the searched stay's quotes. */
export default async function Page({ params, searchParams }: Props) {
  const [{ slug }, raw] = await Promise.all([params, searchParams]);
  const search = parseSearch(raw);
  const queryClient = makeQueryClient();
  try {
    await queryClient.fetchQuery(catalogQueries.room(slug));
  } catch (error) {
    if (isApiError(error) && error.status === 404) notFound();
    // Other failures: render anyway; the client shows the error with a retry.
  }
  const month = firstOfMonth(search.checkIn);
  await Promise.all([
    queryClient.prefetchQuery(catalogQueries.availability(toAvailabilityParams(search))),
    queryClient.prefetchQuery(catalogQueries.calendar(slug, { from: month, to: addMonths(month, 1), plan: CALENDAR_PLAN })),
  ]);
  return (
    <Hydrate client={queryClient}>
      <RoomPage slug={slug} />
    </Hydrate>
  );
}
