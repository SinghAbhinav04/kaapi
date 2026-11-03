import type { Metadata } from 'next';

import { PageShell } from '@/features/site';

import styles from './privacy.module.css';

export const metadata: Metadata = { title: 'How we handle your data' };

/** Plain-language notice under India's DPDP Act, 2023. A lawyer should review the final text. */
export default function Page() {
  return (
    <PageShell>
      <h1 className={styles.title}>
        How we handle <span className="accent">your data</span>
      </h1>
      <div className={styles.body}>
        <h2>What we collect</h2>
        <p>
          To confirm a stay we ask for your name, email, mobile number and country, and anything you choose to tell us about your arrival. We never ask for
          passport details online; guests from outside India show their passport and visa at check-in, as the law requires.
        </p>
        <h2>Why</h2>
        <p>
          Only to manage your booking and contact you about it. If you tick the offers box, we may also send occasional offers; you can unsubscribe any time.
        </p>
        <h2>Enquiries and our AI assistant</h2>
        <p>
          Messages you send us are read by an AI assistant that suggests who should reply and drafts an answer. Before it sees a message, your name, email
          and phone number are removed. A person always reviews and sends the reply. Prices in replies come from our rate table, never from the AI.
        </p>
        <h2>Payments</h2>
        <p>Card details go straight to the payment provider. We never see or store them.</p>
        <h2>How long we keep it</h2>
        <p>
          When your stay is over, your booking moves to our archive. Twelve months after check-out we remove your email address, phone number and any
          notes, and keep only what a tax invoice needs (your name, the dates and the amounts, including GST). Indian law asks us to keep those
          accounts for eight financial years, after which the booking is deleted.
        </p>
        <p>
          Messages you send us, and our replies, are deleted after 12 months. Bookings that were never paid for are deleted after 90 days. A job runs
          several times a day to apply these rules.
        </p>
        <h2>Your rights</h2>
        <p>
          You can ask to see, correct or delete your data, or withdraw consent, by writing to us through the contact form. We reply within 30 days.
        </p>
      </div>
    </PageShell>
  );
}
