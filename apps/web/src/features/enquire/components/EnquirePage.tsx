'use client';

import type { FormEvent } from 'react';

import { PageShell } from '@/features/site';
import { Button, Segmented, SelectField, TextAreaField, TextField } from '@/shared/ui';

import { ENQUIRY_KINDS, EVENT_TYPES } from '../const';
import { useEnquiryForm } from '../hooks';
import type { EnquiryKind } from '../types';
import styles from './EnquirePage.module.css';

export function EnquirePage({ initialKind }: { initialKind: EnquiryKind }) {
  const { form, update, submit, errors, error, sending, reference, reset } = useEnquiryForm(initialKind);
  const isEvent = form.kind === 'EVENT';
  const onSubmit = (e: FormEvent) => {
    e.preventDefault();
    submit();
  };

  if (reference) {
    return (
      <PageShell>
        <section className={styles.sent} role="status">
          <p className="label">RECEIVED · {reference}</p>
          <h1 className={styles.title}>
            Thank you, <span className="accent">we&rsquo;ve got it.</span>
          </h1>
          <p>
            The right person on our team will reply, usually within 30 minutes during working hours (9 am to 9 pm). Quote <span className="mono">{reference}</span>{' '}
            if you write again.
          </p>
          <Button variant="outline" pill onClick={reset}>
            Send another message
          </Button>
        </section>
      </PageShell>
    );
  }

  return (
    <PageShell>
      <div>
        <h1 className={styles.title}>
          {isEvent ? 'Plan an ' : 'Ask the '}
          <span className="accent">{isEvent ? 'event.' : 'team.'}</span>
        </h1>
        <p className={styles.lead}>
          {isEvent
            ? 'Tell us your dates, guest count and budget. Our events team replies with availability and a quote.'
            : 'Questions about rooms, dates, food or getting here. A person replies, not a bot.'}
        </p>
      </div>

      <form className={styles.form} onSubmit={onSubmit} noValidate>
        <Segmented label="What is it about?" options={ENQUIRY_KINDS} value={form.kind} onChange={(kind) => update('kind', kind)} />

        <div className={styles.grid}>
          <TextField label="Your name" autoComplete="name" value={form.name} onChange={(e) => update('name', e.target.value)} />
          <TextField label="Email" type="email" autoComplete="email" value={form.email} onChange={(e) => update('email', e.target.value)} error={errors.email} />
          <TextField label="Phone or WhatsApp" type="tel" autoComplete="tel" optional value={form.phone} onChange={(e) => update('phone', e.target.value)} error={errors.phone} />
        </div>

        {isEvent && (
          <div className={styles.grid}>
            <SelectField label="Kind of event" options={EVENT_TYPES} value={form.eventType} onChange={(e) => update('eventType', e.target.value)} />
            <TextField label="Month" type="month" optional value={form.month} onChange={(e) => update('month', e.target.value)} />
            <TextField label="Guests" inputMode="numeric" optional value={form.guests} onChange={(e) => update('guests', e.target.value)} error={errors.guests} />
            <TextField label="Nights" inputMode="numeric" optional value={form.nights} onChange={(e) => update('nights', e.target.value)} />
            <TextField label="Budget (₹)" inputMode="numeric" optional hint="A rough total is fine." value={form.budget} onChange={(e) => update('budget', e.target.value)} />
          </div>
        )}

        <TextAreaField
          label={isEvent ? 'Anything else we should know?' : 'Your message'}
          optional={isEvent}
          rows={5}
          value={form.message}
          onChange={(e) => update('message', e.target.value)}
          error={errors.message}
        />

        <p className={styles.fine}>
          We use these details only to reply to you. Before your message is read by our AI assistant to route it, your name, email and number are removed.
        </p>
        {error && (
          <p className={styles.error} role="alert">
            {error.message}
          </p>
        )}
        <Button type="submit" size="lg" loading={sending}>
          Send
        </Button>
      </form>
    </PageShell>
  );
}
