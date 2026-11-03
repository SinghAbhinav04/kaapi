'use client';

import Link from 'next/link';

import { useRoom, type Room } from '@/apis/catalog';
import { roomFacts } from '@/features/home';
import { SiteFooter } from '@/features/site';
import { searchToQuery, useStaySearch } from '@/features/stay-search';
import { ROUTES, SITE } from '@/shared/const';
import { formatStayShort, nightsBetween } from '@/shared/lib/dates';
import { formatINR } from '@/shared/lib/money';
import { plural } from '@/shared/lib/text';
import { Button, ErrorState } from '@/shared/ui';

import { useRoomBooking } from '../hooks';
import { Gallery } from './Gallery';
import { PriceCalendar } from './PriceCalendar';
import { RatePlans } from './RatePlans';
import { RoomHeader } from './RoomHeader';
import styles from './RoomPage.module.css';
import { StaySummary } from './StaySummary';

export function RoomPage({ slug }: { slug: string }) {
  const room = useRoom(slug);
  if (room.isError) {
    return (
      <main className={styles.page}>
        <ErrorState message={room.error.message} onRetry={() => room.refetch()} retrying={room.isFetching} />
      </main>
    );
  }
  if (!room.data) return null; // prefetched on the server; only absent if the API failed there too
  return <RoomBody room={room.data.room} />;
}

function RoomBody({ room }: { room: Room }) {
  const { search } = useStaySearch();
  const booking = useRoomBooking(room, search);
  const nights = nightsBetween(search.checkIn, search.checkOut);
  const [first, ...rest] = room.name.split(' ');
  const total = booking.option?.status === 'AVAILABLE' ? booking.plan?.quote.total_paise : undefined;
  const staysHref = ROUTES.staysWith(searchToQuery(search));

  return (
    <>
      <RoomHeader />
      <main className={styles.page}>
        <nav aria-label="Breadcrumb" className={styles.crumbs}>
          <Link href={ROUTES.home}>HOME</Link> / <Link href={staysHref}>STAYS</Link> / <Link href={staysHref}>{room.kind === 'VILLA' ? 'VILLAS' : 'ROOMS'}</Link> /{' '}
          <span aria-current="page">{room.name.toUpperCase()}</span>
        </nav>
        <Gallery photos={room.photos} name={room.name} />

        <div className={styles.layout}>
          <div className={styles.main}>
            <div>
              <h1 className={styles.title}>
                {first} <span className="accent">{rest.join(' ')}</span>
              </h1>
              <p className={styles.facts}>
                {roomFacts(room)}
                {room.amenities.includes('Pet friendly') ? ' · PET FRIENDLY' : ''}
              </p>
              <p className={styles.description}>{room.description}</p>
              <ul className={styles.amenities}>
                {room.amenities.map((a) => (
                  <li key={a}>{a}</li>
                ))}
              </ul>
            </div>

            <PriceCalendar slug={room.slug} />

            {booking.option?.status === 'AVAILABLE' && booking.option.plans.length > 0 && (
              <RatePlans
                plans={booking.option.plans}
                catalogue={room.plans}
                selected={booking.planCode}
                checkIn={search.checkIn}
                onSelect={booking.setPlanCode}
              />
            )}

            <section aria-labelledby="know-title">
              <h2 id="know-title" className={styles.sectionTitle}>
                Good to know
              </h2>
              <p className={styles.description}>
                Check-in from {SITE.checkIn}, check-out by {SITE.checkOut}. A child up to 11 stays free using existing bedding. One extra adult can stay on
                a rollaway bed for {formatINR(room.extra_adult_paise)} a night.
              </p>
            </section>
          </div>

          <StaySummary
            roomName={room.name}
            search={search}
            option={booking.option}
            plan={booking.plan}
            loading={booking.availability.isPending}
            updating={booking.availability.isPlaceholderData}
            promo={booking.promo}
            onPromo={booking.setPromo}
            onReserve={booking.reserve}
            reserving={booking.reserving}
            error={booking.reserveError}
          />
        </div>
      </main>
      <SiteFooter />

      <div className={styles.mobileBar}>
        <div>
          <p className={styles.mobileTotal}>{total !== undefined ? formatINR(total) : '—'}</p>
          <p className={styles.mobileMeta}>
            {formatStayShort(search.checkIn, search.checkOut)} · {plural(nights, 'night')} · incl. GST
          </p>
        </div>
        <Button onClick={booking.reserve} loading={booking.reserving} disabled={total === undefined}>
          Reserve
        </Button>
      </div>
    </>
  );
}
