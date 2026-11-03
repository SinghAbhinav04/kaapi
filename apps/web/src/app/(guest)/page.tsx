import { catalogQueries } from '@/apis/catalog';
import { Hydrate, makeQueryClient } from '@/apis/query';
import { HomePage } from '@/features/home';
import { parseSearch, toAvailabilityParams } from '@/features/stay-search';

interface Props {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}

/**
 * Server-rendered with the rooms and live prices already in the HTML. The client cache is
 * filled from the same data, so nothing is fetched twice; prices then refresh in the
 * background when they go stale. prefetchQuery never throws: if the API is down, the page
 * still renders and the client shows the error state with a retry.
 */
export default async function Page({ searchParams }: Props) {
  const search = parseSearch(await searchParams);
  const queryClient = makeQueryClient();
  await Promise.all([
    queryClient.prefetchQuery(catalogQueries.rooms()),
    queryClient.prefetchQuery(catalogQueries.availability(toAvailabilityParams(search))),
  ]);
  return (
    <Hydrate client={queryClient}>
      <HomePage />
    </Hydrate>
  );
}
