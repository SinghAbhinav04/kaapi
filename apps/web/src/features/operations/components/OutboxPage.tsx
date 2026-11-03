'use client';

import { useState } from 'react';

import { useCan } from '@/apis/auth';
import { useOutbox } from '@/apis/operations';
import { Card, StaffPageHeader } from '@/features/staff-shell';
import { formatTimestamp } from '@/shared/lib/dates';
import { Button, EmptyState, ErrorState, Pill, Skeleton } from '@/shared/ui';

import { OUTBOX_KIND, OUTBOX_STATUS } from '../const';
import { ComposeDialog } from './ComposeDialog';
import styles from './Operations.module.css';

/** Everything the system sent. Email goes out through Gmail once connected; the rest is simulated. */
export function OutboxPage() {
  const outbox = useOutbox();
  const canSend = useCan('enquiries:send');
  const [composing, setComposing] = useState(false);
  return (
    <div className={styles.page}>
      <StaffPageHeader
        title="Outbox"
        description="Every message the system has sent. Email goes out through the connected Gmail mailbox; anything marked Simulated was only recorded here."
        actions={
          canSend && (
            <Button variant="ink" onClick={() => setComposing(true)}>
              New message
            </Button>
          )
        }
      />
      <ComposeDialog open={composing} onClose={() => setComposing(false)} />
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
              const kind = OUTBOX_KIND[m.kind] ?? {
                label: m.kind,
                tone: 'neutral' as const,
              };
              const delivery = OUTBOX_STATUS[m.status] ?? {
                label: m.status,
                tone: 'neutral' as const,
              };
              return (
                <li key={m.id}>
                  <details className={styles.message}>
                    <summary>
                      <Pill tone={kind.tone}>{kind.label}</Pill>
                      <span className={styles.recipient}>
                        {m.channel.toLowerCase()} → {m.recipient}
                      </span>
                      <span className={styles.subject}>{m.subject ?? m.body.slice(0, 80)}</span>
                      <span className={styles.time}>
                        <Pill tone={delivery.tone}>{delivery.label}</Pill> {formatTimestamp(m.created_at)}
                      </span>
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
