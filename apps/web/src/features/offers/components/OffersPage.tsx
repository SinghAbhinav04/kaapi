'use client';

import { useState } from 'react';

import { useOffers, type Offer } from '@/apis/offers';
import { Card, StaffPageHeader } from '@/features/staff-shell';
import { formatDayMonth, todayIST } from '@/shared/lib/dates';
import { Button, EmptyState, ErrorState, Pill, Skeleton } from '@/shared/ui';

import { COPY, STATE_PILL } from '../const';
import { describeValue, offerState } from '../lib';
import { CampaignsCard } from './CampaignsCard';
import { OfferDialog } from './OfferDialog';
import styles from './Offers.module.css';

export function OffersPage() {
  const offers = useOffers();
  const [editing, setEditing] = useState<{ offer?: Offer } | null>(null);
  const today = todayIST();

  return (
    <div className={styles.page}>
      <StaffPageHeader title={COPY.title} description={COPY.description} actions={<Button onClick={() => setEditing({})}>{COPY.add}</Button>} />
      <Card className={styles.card}>
        {offers.isPending ? (
          <Skeleton height={160} />
        ) : offers.isError ? (
          <ErrorState message={offers.error.message} requestId={offers.error.requestId} onRetry={() => void offers.refetch()} retrying={offers.isFetching} />
        ) : offers.data.offers.length === 0 ? (
          <EmptyState title="No offers" compact>
            {COPY.empty}
          </EmptyState>
        ) : (
          <div className={styles.scroller}>
            <table className={styles.table}>
              <thead>
                <tr>
                  <th scope="col">Offer</th>
                  <th scope="col">Discount</th>
                  <th scope="col">Stays</th>
                  <th scope="col">Bookable</th>
                  <th scope="col">Used</th>
                  <th scope="col">Status</th>
                  <th scope="col">
                    <span className="sr-only">Actions</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {offers.data.offers.map((o) => {
                  const state = STATE_PILL[offerState(o, today)];
                  return (
                    <tr key={o.id}>
                      <td>
                        <span className="mono">{o.code}</span>
                        <span className={styles.sub}>
                          {o.auto_apply ? 'Automatic · ' : 'Code · '}
                          {o.description}
                        </span>
                      </td>
                      <td className="mono">{describeValue(o)}</td>
                      <td className="mono">
                        {formatDayMonth(o.stay_from)} – {formatDayMonth(o.stay_to)}
                      </td>
                      <td className="mono">
                        {formatDayMonth(o.book_from)} – {formatDayMonth(o.book_to)}
                      </td>
                      <td className="mono">
                        {o.used_count}
                        {o.max_uses !== null ? ` / ${o.max_uses}` : ''}
                      </td>
                      <td>
                        <Pill tone={state.tone} mono>
                          {state.label}
                        </Pill>
                      </td>
                      <td>
                        <Button variant="quiet" onClick={() => setEditing({ offer: o })}>
                          Edit
                        </Button>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </Card>
      <CampaignsCard />
      <OfferDialog editing={editing} onClose={() => setEditing(null)} />
    </div>
  );
}
