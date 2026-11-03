'use client';

import { useOutbox } from '@/apis/operations';
import { Card, StaffPageHeader } from '@/features/staff-shell';
import { formatTimestamp } from '@/shared/lib/dates';
import { EmptyState, ErrorState, Pill, Skeleton } from '@/shared/ui';

import { OUTBOX_KIND } from '../const';
import styles from './Operations.module.css';

/** Everything the system "sent". Email, WhatsApp and SMS are simulated in this build. */
export function OutboxPage() {
  const outbox = useOutbox();
  return (
    <div className={styles.page}>
      <StaffPageHeader title="Outbox" description="Every message the system has sent. Channels are simulated here; in production they go to the email and WhatsApp providers." />
      <Card className={styles.card}>
        {outbox.isPending ? (
          <Skeleton height={300} radius={12} />
        ) : outbox.isError ? (
          <ErrorState message={outbox.error.message} onRetry={() => outbox.refetch()} retrying={outbox.isFetching} />
        ) : outbox.data.messages.length === 0 ? (
          <EmptyState compact title="Nothing sent yet">
            Confirmations, replies and alerts appear here as they go out.
          </EmptyState>
        ) : (
          <ul className={styles.messages}>
            {outbox.data.messages.map((m) => {
              const kind = OUTBOX_KIND[m.kind] ?? { label: m.kind, tone: 'neutral' as const };
              return (
                <li key={m.id}>
                  <details className={styles.message}>
                    <summary>
                      <Pill tone={kind.tone}>{kind.label}</Pill>
                      <span className={styles.recipient}>
                        {m.channel.toLowerCase()} → {m.recipient}
                      </span>
                      <span className={styles.subject}>{m.subject ?? m.body.slice(0, 80)}</span>
                      <span className={styles.time}>{formatTimestamp(m.created_at)}</span>
                    </summary>
                    <pre className={styles.body}>{m.body}</pre>
                  </details>
                </li>
              );
            })}
          </ul>
        )}
      </Card>
    </div>
  );
}
