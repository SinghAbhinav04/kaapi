'use client';

import Link from 'next/link';
import { useState, type FormEvent, type ReactNode } from 'react';

import { HoldTimer, Steps, type HoldTicket } from '@/features/booking-flow';
import { searchToQuery } from '@/features/stay-search';
import { ROUTES } from '@/shared/const';
import { formatINR } from '@/shared/lib/money';
import { Button, ButtonLink, Checkbox, EmptyState, Logo, SelectField, Skeleton, TextAreaField, TextField } from '@/shared/ui';

import { ARRIVAL_WINDOWS, COUNTRIES } from '../const';
import { useCheckout, useHoldTicket } from '../hooks';
import styles from './CheckoutPage.module.css';
import { OrderSummary } from './OrderSummary';

export function CheckoutPage({ holdId }: { holdId: string }) {
  const ticket = useHoldTicket(holdId);
  if (ticket === undefined) {
    return (
      <Shell>
        <Skeleton height={420} radius={16} />
      </Shell>
    );
  }
  if (ticket === null) {
    return (
      <Shell>
        <EmptyState
          title="This booking isn't open in this tab"
          action={
            <ButtonLink href={ROUTES.stays} variant="ink" pill>
              Choose a stay
            </ButtonLink>
          }
        >
          Holds belong to the browser tab that made them, and end after 10 minutes. Pick your room again and we&rsquo;ll hold it for you.
        </EmptyState>
      </Shell>
    );
  }
  return <CheckoutForm ticket={ticket} />;
}

function roomHref(ticket: HoldTicket) {
  const { stay } = ticket.hold;
  return ROUTES.room(
    ticket.roomSlug,
    searchToQuery({ checkIn: stay.check_in, checkOut: stay.check_out, adults: ticket.adults, children: ticket.children, rooms: ticket.hold.rooms }),
  );
}

function Shell({ children, ticket, onExpire }: { children: ReactNode; ticket?: HoldTicket; onExpire?: () => void }) {
  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <div className={styles.headerInner}>
          <Logo />
          <Steps current={2} roomHref={ticket ? roomHref(ticket) : undefined} />
          {ticket && <HoldTimer expiresAt={ticket.hold.expires_at} onExpire={onExpire} />}
        </div>
      </header>
      <div className={styles.body}>{children}</div>
    </div>
  );
}

function CheckoutForm({ ticket }: { ticket: HoldTicket }) {
  const { form, update, submit, errors, formError, submitting } = useCheckout(ticket);
  const [expired, setExpired] = useState(() => new Date(ticket.hold.expires_at).getTime() <= Date.now());
  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    submit();
  };

  return (
    <Shell ticket={ticket} onExpire={() => setExpired(true)}>
      <form aria-labelledby="details-title" className={styles.form} onSubmit={onSubmit} noValidate>
        <div>
          <h1 id="details-title" className={styles.title}>
            Who&rsquo;s <span className="accent">coming?</span>
          </h1>
          <p className={styles.lead}>We only ask for what we need to confirm your stay and reach you about it.</p>
        </div>

        {expired && (
          <div className={styles.expired} role="alert">
            <strong>Your 10-minute hold has ended.</strong> The room may still be free.{' '}
            <Link href={roomHref(ticket)}>Check and hold it again</Link>
          </div>
        )}

        <div className={styles.fields}>
          <TextField label="Full name" autoComplete="name" value={form.name} onChange={(e) => update('name', e.target.value)} error={errors['guest.name']} required />
          <TextField
            label="Email"
            type="email"
            autoComplete="email"
            inputMode="email"
            value={form.email}
            onChange={(e) => update('email', e.target.value)}
            hint="Your confirmation goes here."
            error={errors['guest.email']}
            required
          />
          <TextField
            label="Mobile"
            type="tel"
            autoComplete="tel"
            inputMode="tel"
            placeholder="+91 98450 12345"
            value={form.phone}
            onChange={(e) => update('phone', e.target.value)}
            error={errors['guest.phone']}
            required
          />
          <SelectField
            label="Country of residence"
            autoComplete="country"
            options={COUNTRIES}
            value={form.country}
            onChange={(e) => update('country', e.target.value)}
            error={errors['guest.country']}
          />
          <SelectField label="Arriving around" options={ARRIVAL_WINDOWS} value={form.arrivalWindow} onChange={(e) => update('arrivalWindow', e.target.value)} />
        </div>

        <TextAreaField
          label="Anything we should know?"
          optional
          rows={3}
          value={form.specialRequests}
          onChange={(e) => update('specialRequests', e.target.value)}
          error={errors.special_requests}
        />

        <div className={styles.consent}>
          <Checkbox checked={form.consent} onChange={(e) => update('consent', e.target.checked)} aria-invalid={errors.consent ? true : undefined}>
            Kaapi Estate may use my name, email and mobile to manage this booking and contact me about it.{' '}
            <Link href={ROUTES.privacy}>How we handle your data</Link>
          </Checkbox>
          {errors.consent && (
            <p className={styles.fieldError} role="alert">
              {errors.consent}
            </p>
          )}
          <Checkbox muted checked={form.marketing} onChange={(e) => update('marketing', e.target.checked)}>
            Send me occasional offers. Optional, and you can unsubscribe any time.
          </Checkbox>
          <p className={styles.passport}>We never ask for passport details online. Guests from outside India show their passport and visa at check-in.</p>
        </div>

        {formError && (
          <p className={styles.fieldError} role="alert">
            {formError.code === 'HOLD_EXPIRED' || formError.code === 'HOLD_NOT_FOUND' ? 'Your hold has ended. Check the room again to hold it.' : formError.message}
          </p>
        )}

        <div className={styles.actions}>
          <Button type="submit" size="lg" loading={submitting} disabled={expired}>
            Continue to payment · {formatINR(ticket.hold.quote.total_paise)}
          </Button>
          <span className={styles.muted}>Test mode: no real money moves.</span>
        </div>
      </form>

      <OrderSummary ticket={ticket} />
    </Shell>
  );
}
