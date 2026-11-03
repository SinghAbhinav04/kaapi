'use client';

import { useRouter } from 'next/navigation';
import { useEffect } from 'react';

import { usePaymentPage, useSimulatePayment, type PaymentOutcome } from '@/apis/payments';
import { HoldTimer, Steps } from '@/features/booking-flow';
import { ROUTES } from '@/shared/const';
import { formatStay } from '@/shared/lib/dates';
import { formatINR } from '@/shared/lib/money';
import { plural } from '@/shared/lib/text';
import { Button, ButtonLink, ErrorState, Logo, Pill, Skeleton } from '@/shared/ui';

import { PAID_STATUSES } from '../const';
import styles from './PaymentPage.module.css';

/**
 * Step 3, the mock gateway. In production this is the provider's hosted page; here the two
 * buttons make the API sign and deliver a real webhook, exactly as the provider would.
 */
export function PaymentPage({ orderId }: { orderId: string }) {
  const router = useRouter();
  const page = usePaymentPage(orderId);
  const simulate = useSimulatePayment(orderId);
  const data = page.data;
  const paid = data ? PAID_STATUSES.has(data.status) || data.booking_status === 'CONFIRMED' : false;

  // Once captured, the confirmation page takes over (it also explains a late payment).
  useEffect(() => {
    if (paid && data) router.replace(ROUTES.booking(data.booking_ref));
  }, [paid, data, router]);

  const pay = (outcome: PaymentOutcome) => simulate.mutate(outcome);

  return (
    <div className={styles.page}>
      <header className={styles.header}>
        <div className={styles.headerInner}>
          <Logo />
          <Steps current={3} />
          {data?.hold_expires_at && !paid && <HoldTimer expiresAt={data.hold_expires_at} />}
        </div>
      </header>

      <main className={styles.main}>
        {page.isPending ? (
          <Skeleton height={380} radius={20} />
        ) : page.isError ? (
          <ErrorState message={page.error.message} requestId={page.error.requestId} onRetry={() => page.refetch()} retrying={page.isFetching} />
        ) : (
          data && (
            <section aria-labelledby="pay-title" className={styles.gateway}>
              <div className={styles.gatewayHead}>
                <span className="label">KAAPI ESTATE · SECURE CHECKOUT</span>
                <Pill tone="unsorted" mono>
                  TEST MODE
                </Pill>
              </div>
              <h1 id="pay-title" className={styles.amount}>
                {formatINR(data.amount_paise)}
              </h1>
              <p className={styles.detail}>
                {data.room_name} · {data.plan_name}
                <br />
                {formatStay(data.stay.check_in, data.stay.check_out)} · {plural(data.stay.nights, 'night')} · booking {data.booking_ref}
              </p>

              {data.status === 'FAILED' && (
                <p className={styles.failed} role="alert">
                  The payment didn&rsquo;t go through{data.failure_reason ? `: ${data.failure_reason}` : ''}. Your room is still held; you can try again.
                </p>
              )}
              {simulate.isError && (
                <p className={styles.failed} role="alert">
                  {simulate.error.message}
                </p>
              )}

              {paid ? (
                <p className={styles.done} role="status">
                  Payment received. Opening your confirmation…
                </p>
              ) : data.booking_status === 'EXPIRED' ? (
                <div className={styles.done}>
                  <p>This booking expired before it was paid. Nothing was charged.</p>
                  <ButtonLink href={ROUTES.stays} variant="ink" pill>
                    Choose dates again
                  </ButtonLink>
                </div>
              ) : (
                <div className={styles.buttons}>
                  <Button size="lg" block onClick={() => pay('SUCCESS')} loading={simulate.isPending && simulate.variables === 'SUCCESS'} disabled={simulate.isPending}>
                    Pay {formatINR(data.amount_paise)}
                  </Button>
                  <Button variant="outline" block onClick={() => pay('FAILURE')} loading={simulate.isPending && simulate.variables === 'FAILURE'} disabled={simulate.isPending}>
                    Simulate a declined payment
                  </Button>
                </div>
              )}
              <p className={styles.fine}>No real money moves. The gateway signs a webhook to our API, which confirms the booking only after verifying it.</p>
            </section>
          )
        )}
      </main>
    </div>
  );
}
