'use client';

import Link from 'next/link';
import type { FormEvent } from 'react';

import { useLogout, useMe } from '@/apis/auth';
import { useMyBookings, type Booking } from '@/apis/booking';
import { PageShell } from '@/features/site';
import { ROUTES } from '@/shared/const';
import { formatStay } from '@/shared/lib/dates';
import { formatINR } from '@/shared/lib/money';
import { formatParty } from '@/shared/lib/text';
import { Button, ButtonLink, EmptyState, ErrorState, Pill, Skeleton, TextField } from '@/shared/ui';

import { OTP_LENGTH, STATUS_LABEL } from '../const';
import { useOtpSignIn } from '../hooks';
import styles from './MyBookingsPage.module.css';

export function MyBookingsPage() {
  const me = useMe();
  const user = me.data;
  return (
    <PageShell>
      <div>
        <h1 className={styles.title}>
          My <span className="accent">bookings</span>
        </h1>
        <p className={styles.lead}>Sign in with a one-time code sent to your email. No password needed.</p>
      </div>
      {me.isPending ? (
        <Skeleton height={200} radius={16} />
      ) : user?.role === 'GUEST' ? (
        <BookingList email={user.email} />
      ) : user ? (
        <EmptyState title={`You're signed in as staff (${user.name})`} action={<ButtonLink href={ROUTES.staff.inbox}>Open the staff console</ButtonLink>}>
          Guest bookings show here for guests. Staff see every booking in the console.
        </EmptyState>
      ) : (
        <SignIn />
      )}
    </PageShell>
  );
}

function SignIn() {
  const otp = useOtpSignIn();
  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    if (otp.sent) otp.verifyCode();
    else otp.sendCode();
  };
  return (
    <form className={styles.card} onSubmit={onSubmit}>
      {!otp.sent ? (
        <TextField label="Email you booked with" type="email" autoComplete="email" inputMode="email" value={otp.email} onChange={(e) => otp.setEmail(e.target.value)} required />
      ) : (
        <>
          <p className={styles.sentTo}>
            We sent a {OTP_LENGTH}-digit code to <strong>{otp.email}</strong>. It works for 5 minutes.{' '}
            <button type="button" className={styles.linkButton} onClick={otp.restart}>
              Use another email
            </button>
          </p>
          {otp.demoCode && (
            <p className={styles.demo} role="status">
              Demo: email is simulated, so here is your code: <span className="mono">{otp.demoCode}</span>
            </p>
          )}
          <TextField
            label="One-time code"
            inputMode="numeric"
            autoComplete="one-time-code"
            pattern="[0-9]*"
            maxLength={OTP_LENGTH}
            value={otp.code}
            onChange={(e) => otp.setCode(e.target.value.replace(/\D/g, ''))}
            required
          />
        </>
      )}
      {otp.error && (
        <p className={styles.error} role="alert">
          {otp.error.message}
        </p>
      )}
      <Button type="submit" loading={otp.sending || otp.verifying} disabled={otp.sent && otp.code.length !== OTP_LENGTH}>
        {otp.sent ? 'Sign in' : 'Send me a code'}
      </Button>
    </form>
  );
}

function BookingList({ email }: { email: string }) {
  const bookings = useMyBookings();
  const logout = useLogout();
  return (
    <>
      <p className={styles.signedIn}>
        Signed in as {email}.{' '}
        <button type="button" className={styles.linkButton} onClick={() => logout.mutate()}>
          Sign out
        </button>
      </p>
      {bookings.isPending ? (
        <Skeleton height={140} radius={16} />
      ) : bookings.isError ? (
        <ErrorState message={bookings.error.message} onRetry={() => bookings.refetch()} retrying={bookings.isFetching} />
      ) : bookings.data.bookings.length === 0 ? (
        <EmptyState title="No bookings under this email yet" action={<ButtonLink href={ROUTES.stays}>See stays and prices</ButtonLink>}>
          If you booked with a different email, sign out and use that one.
        </EmptyState>
      ) : (
        <ul className={styles.list}>
          {bookings.data.bookings.map((b) => (
            <BookingRow key={b.ref} booking={b} />
          ))}
        </ul>
      )}
    </>
  );
}

function BookingRow({ booking }: { booking: Booking }) {
  const status = STATUS_LABEL[booking.status];
  return (
    <li>
      <Link href={ROUTES.booking(booking.ref)} className={styles.row}>
        <span className={styles.rowMain}>
          <strong>{booking.room.name}</strong>
          <span className={styles.muted}>
            {formatStay(booking.stay.check_in, booking.stay.check_out)} · {formatParty(booking.adults, booking.children)}
          </span>
          <span className={`mono ${styles.muted}`}>{booking.ref}</span>
        </span>
        <span className={styles.rowSide}>
          <Pill tone={status.tone}>{status.label}</Pill>
          <span className="mono">{formatINR(booking.quote.total_paise)}</span>
        </span>
      </Link>
    </li>
  );
}
