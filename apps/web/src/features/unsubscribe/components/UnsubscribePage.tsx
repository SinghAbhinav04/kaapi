'use client';

import { useUnsubscribe } from '@/apis/marketing';
import { ROUTES } from '@/shared/const';
import { Button, ButtonLink } from '@/shared/ui';

import styles from './UnsubscribePage.module.css';

/**
 * The link in every offers email. It asks for one click rather than unsubscribing on load,
 * because mail scanners open links on their own.
 */
export function UnsubscribePage({ token }: { token: string }) {
  const unsubscribe = useUnsubscribe();

  return (
    <section className={styles.wrap}>
      <h1 className={styles.title}>
        Offer <span className="accent">emails</span>
      </h1>
      {unsubscribe.isSuccess ? (
        <p className={styles.lead} role="status">
          Done. You won&rsquo;t get offer emails from us any more. Booking confirmations and reminders about your stays still arrive.
        </p>
      ) : !token ? (
        <p className={styles.lead}>This link is missing its code. Use the unsubscribe link at the bottom of any offer email.</p>
      ) : (
        <>
          <p className={styles.lead}>Stop offers and seasonal emails from Kaapi Estate? Emails about your bookings aren&rsquo;t affected.</p>
          {unsubscribe.isError && (
            <p className={styles.error} role="alert">
              {unsubscribe.error.message}
            </p>
          )}
          <Button size="lg" onClick={() => unsubscribe.mutate(token)} loading={unsubscribe.isPending}>
            Unsubscribe
          </Button>
        </>
      )}
      <ButtonLink href={ROUTES.stays} variant="outline" pill>
        See the rooms
      </ButtonLink>
    </section>
  );
}
