'use client';

import Link from 'next/link';

import { useBooking, type Booking } from '@/apis/booking';
import { STORAGE_KEY, cancellationLine } from '@/features/booking-flow';
import { ROUTES, SITE } from '@/shared/const';
import { useSessionValue } from '@/shared/hooks';
import { formatDay } from '@/shared/lib/dates';
import { formatINR } from '@/shared/lib/money';
import { firstName, formatParty } from '@/shared/lib/text';
import { Button, ButtonLink, EmptyState, ErrorState, Logo, Pill, Skeleton } from '@/shared/ui';

import { downloadFile, stayCalendarFile } from '../lib';
import styles from './ConfirmationPage.module.css';

export function ConfirmationPage({ bookingRef }: { bookingRef: string }) {
  const token = useSessionValue<string>(STORAGE_KEY.booking(bookingRef));
  const booking = useBooking(bookingRef, token ?? undefined, token !== undefined);

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <Logo />
        <Pill tone="unsorted" mono>
          TEST MODE
        </Pill>
      </header>
      <main className={styles.main}>
        {booking.isPending ? (
          <Skeleton height={320} radius={20} />
        ) : booking.isError ? (
          booking.error.status === 401 || booking.error.status === 403 || booking.error.status === 404 ? (
            <EmptyState
              title="Sign in to see this booking"
              action={
                <ButtonLink href={ROUTES.myBookings} variant="ink" pill>
                  Sign in with a one-time code
                </ButtonLink>
              }
            >
              For your privacy, a booking opens only in the browser that made it, or after you sign in with a code sent to your email.
            </EmptyState>
          ) : (
            <ErrorState message={booking.error.message} requestId={booking.error.requestId} onRetry={() => booking.refetch()} retrying={booking.isFetching} />
          )
        ) : (
          <Status booking={booking.data.booking} />
        )}
      </main>
    </div>
  );
}

function Status({ booking }: { booking: Booking }) {
  switch (booking.status) {
    case 'CONFIRMED':
      return <Confirmed booking={booking} />;
    case 'PENDING_PAYMENT':
      return (
        <section className={styles.notice} role="status" aria-live="polite">
          <h1 className={styles.noticeTitle}>Confirming your payment…</h1>
          <p>This usually takes a few seconds. You can keep this page open.</p>
          {booking.payment && (
            <Link href={ROUTES.pay(booking.payment.order_id)}>Haven&rsquo;t paid yet? Go to payment</Link>
          )}
        </section>
      );
    case 'PAYMENT_FAILED':
      return (
        <section className={styles.notice}>
          <h1 className={styles.noticeTitle}>The payment didn&rsquo;t go through</h1>
          <p>Nothing was charged. {booking.payment?.failure_reason}</p>
          {booking.payment && (
            <ButtonLink href={ROUTES.pay(booking.payment.order_id)} variant="primary">
              Try paying again
            </ButtonLink>
          )}
        </section>
      );
    case 'REFUND_PENDING':
    case 'REFUNDED':
      return (
        <section className={styles.notice}>
          <h1 className={styles.noticeTitle}>We couldn&rsquo;t keep the room, so we&rsquo;re refunding you</h1>
          <p>
            Your payment arrived after the 10-minute hold ended, and the last room had been booked by then. The full {formatINR(booking.quote.total_paise)}{' '}
            {booking.status === 'REFUNDED' ? 'has been refunded' : 'is being refunded'} to the card you used. Our team has been told and may call you with other
            dates.
          </p>
          <ButtonLink href={ROUTES.stays} variant="ink" pill>
            See other dates
          </ButtonLink>
        </section>
      );
    default:
      return (
        <section className={styles.notice}>
          <h1 className={styles.noticeTitle}>This booking is {booking.status.toLowerCase().replace('_', ' ')}</h1>
          <p>Nothing was charged.</p>
          <ButtonLink href={ROUTES.stays} variant="ink" pill>
            Choose a stay
          </ButtonLink>
        </section>
      );
  }
}

function Confirmed({ booking }: { booking: Booking }) {
  const extras = booking.special_requests ? ` · ${booking.special_requests}` : '';
  return (
    <>
      <section aria-labelledby="ct" className={styles.hero}>
        <div className={`${styles.tick} rise`} aria-hidden="true">
          <svg width="22" height="22" viewBox="0 0 22 22" fill="none" stroke="currentColor" strokeWidth="2.2">
            <path d="M5 11.5l4 4 8-9" />
          </svg>
        </div>
        <h1 id="ct" className={styles.heroTitle}>
          You&rsquo;re booked, <span className="accent">{firstName(booking.guest.name)}.</span>
        </h1>
        <p className={styles.heroText}>We&rsquo;ve sent the details to {booking.guest.email}.</p>
        <div className={styles.refBlock}>
          <p className={styles.refLabel}>BOOKING REFERENCE</p>
          <p className={`${styles.ref} rise`}>{booking.ref}</p>
        </div>
      </section>

      <section aria-labelledby="cs" className={styles.stay}>
        <h2 id="cs" className="label">
          YOUR STAY
        </h2>
        <dl className={styles.facts}>
          <dt>Stay</dt>
          <dd>
            {booking.room.name} · {booking.plan.name}
          </dd>
          <dt>Check-in</dt>
          <dd>
            {formatDay(booking.stay.check_in)}, from {SITE.checkIn}
          </dd>
          <dt>Check-out</dt>
          <dd>
            {formatDay(booking.stay.check_out)}, by {SITE.checkOut}
          </dd>
          <dt>Guests</dt>
          <dd>
            {formatParty(booking.adults, booking.children)}
            {extras}
          </dd>
          <dt>Paid</dt>
          <dd>
            <span className="mono">{formatINR(booking.quote.total_paise)}</span> incl. {formatINR(booking.quote.tax_paise)} GST
          </dd>
          <dt>Cancel</dt>
          <dd className={booking.plan.refundable ? styles.good : undefined}>{cancellationLine(booking.plan.refundable, booking.stay.check_in)}</dd>
        </dl>
      </section>

      <div className={styles.actions}>
        <Button variant="outline" onClick={() => downloadFile(`${booking.ref}.ics`, stayCalendarFile(booking), 'text/calendar')}>
          Add to calendar
        </Button>
        <ButtonLink href={ROUTES.myBookings} variant="ink">
          Manage booking
        </ButtonLink>
      </div>
      <p className={styles.fine}>To manage it later, sign in with a one-time code sent to your email. No password needed.</p>

      <section aria-labelledby="cg" className={styles.getting}>
        <h2 id="cg" className={styles.gettingTitle}>
          Getting <span className="accent">here</span>
        </h2>
        <ol className={styles.route}>
          <li>
            <span>~40 KM</span>Kadur railway station. We can arrange a pick-up.
          </li>
          <li>
            <span>~150 KM</span>Mangaluru airport.
          </li>
          <li>
            <span>5–6 H</span>By road from Bengaluru. Signal drops in the last 10 km, so download the map before you leave.
          </li>
        </ol>
        <p className={styles.fine}>Bring a photo ID for check-in. Pets are welcome in Coffee Villas, but not in the dining areas.</p>
      </section>
    </>
  );
}
