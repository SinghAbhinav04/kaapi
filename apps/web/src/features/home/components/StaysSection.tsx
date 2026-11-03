'use client';

import Link from 'next/link';

import { useAvailability, useRooms } from '@/apis/catalog';
import { nightsOf, searchToQuery, toAvailabilityParams, useStaySearch } from '@/features/stay-search';
import { ROUTES } from '@/shared/const';
import { cx } from '@/shared/lib/cx';
import { formatStayShort } from '@/shared/lib/dates';
import { formatParty } from '@/shared/lib/text';
import { ErrorState, Skeleton } from '@/shared/ui';

import { STAYS_ANCHOR } from '../const';
import { toStayCards } from '../lib';
import { StayCard } from './StayCard';
import styles from './StaysSection.module.css';

/** The rooms with live totals for the searched dates. */
export function StaysSection() {
  const { search } = useStaySearch();
  const rooms = useRooms();
  const availability = useAvailability(toAvailabilityParams(search));
  const cards = toStayCards(rooms.data?.rooms ?? [], availability.data);
  const query = searchToQuery(search);
  const updating = availability.isPlaceholderData;

  return (
    <section data-trail="left" id={STAYS_ANCHOR} aria-labelledby="stays-title" className={styles.section}>
      <div className={`${styles.head} reveal`}>
        <div>
          <p className="label">
            STAYS · {formatStayShort(search.checkIn, search.checkOut).toUpperCase()} · {formatParty(search.adults, search.children).toUpperCase()}
          </p>
          <h2 id="stays-title" className={styles.heading}>
            Five ways to <span className="accent">stay.</span>
          </h2>
        </div>
        <p className={styles.note}>
          Prices are totals for your dates, GST included. &ldquo;Left&rdquo; counts only what we sell on this website.
          {updating && <span className={styles.updating}> Updating prices…</span>}
        </p>
      </div>

      {rooms.isError ? (
        <ErrorState message={rooms.error.message} onRetry={() => rooms.refetch()} retrying={rooms.isFetching} />
      ) : (
        <div className={cx(styles.grid, updating && styles.stale)} aria-busy={availability.isFetching}>
          {rooms.isPending
            ? Array.from({ length: 5 }, (_, i) => <Skeleton key={i} height={420} radius={16} />)
            : cards.map((card) => (
                <StayCard key={card.slug} card={card} nights={nightsOf(search)} query={query} pricesLoading={availability.isPending} />
              ))}
          <div className={`${styles.helper} reveal`}>
            <p className={styles.helperTitle}>Travelling as a family of five, or with a dog?</p>
            <p className={styles.helperBody}>
              Coffee Villas sleep four adults and two children, and pets are welcome there. Tell us who&rsquo;s coming and we&rsquo;ll suggest a fit.
            </p>
            <Link href={ROUTES.enquire}>Ask the reservations team</Link>
          </div>
        </div>
      )}
      {availability.isError && !availability.data && (
        <p className={styles.priceError} role="status">
          We couldn&rsquo;t load prices just now. {availability.error.message}{' '}
          <button type="button" onClick={() => availability.refetch()}>
            Try again
          </button>
        </p>
      )}
    </section>
  );
}
