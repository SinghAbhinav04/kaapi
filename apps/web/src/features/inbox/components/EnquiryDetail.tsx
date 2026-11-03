'use client';

import { hasPermission, useMe } from '@/apis/auth';
import { useEnquiry, useRetryAi, useSetEnquiryStatus, type Enquiry, type EnquiryEvent } from '@/apis/enquiries';
import { formatTimestamp } from '@/shared/lib/dates';
import { initials } from '@/shared/lib/text';
import { Button, EmptyState, ErrorState, Pill, Skeleton } from '@/shared/ui';

import { CATEGORY_LABEL, CATEGORY_TONE, CHANNEL_LABEL, LANE_LABEL } from '../const';
import { AiUnderstood } from './AiUnderstood';
import { DraftPanel } from './DraftPanel';
import styles from './Inbox.module.css';
import { MoveMenu } from './MoveMenu';

export function EnquiryDetail({ enquiryRef, onClose }: { enquiryRef: string | null; onClose: () => void }) {
  const detail = useEnquiry(enquiryRef);
  if (!enquiryRef) {
    return (
      <section className={styles.detailPane}>
        <EmptyState title="Pick a message">Choose an enquiry on the left to see what the AI understood and the reply draft.</EmptyState>
      </section>
    );
  }
  return (
    <section aria-labelledby="enq-title" className={styles.detailPane}>
      <button type="button" className={styles.back} onClick={onClose}>
        All messages
      </button>
      {detail.isPending ? (
        <div className={styles.skeletons}>
          <Skeleton height={52} width="50%" />
          <Skeleton height={80} />
          <Skeleton height={140} />
          <Skeleton height={260} />
        </div>
      ) : detail.isError ? (
        <ErrorState
          message={detail.error.status === 404 ? 'This enquiry is in a lane you can’t see, or it was moved.' : detail.error.message}
          requestId={detail.error.requestId}
          onRetry={detail.error.status === 404 ? undefined : () => detail.refetch()}
        />
      ) : (
        <Detail enquiry={detail.data.enquiry} onMoved={onClose} />
      )}
    </section>
  );
}

function Detail({ enquiry, onMoved }: { enquiry: Enquiry; onMoved: () => void }) {
  const { data: me } = useMe();
  const canRoute = hasPermission(me ?? null, 'enquiries:reroute');
  const canSend = hasPermission(me ?? null, 'enquiries:send');
  const setStatus = useSetEnquiryStatus(enquiry.ref);
  const retry = useRetryAi(enquiry.ref);
  const contact = enquiry.channel === 'WHATSAPP' ? enquiry.sender_phone : (enquiry.sender_email ?? enquiry.sender_phone);
  const closed = enquiry.status === 'CLOSED' || enquiry.status === 'SPAM';

  return (
    <>
      <header className={styles.detailHead}>
        <div className={styles.person}>
          <span className={`${styles.avatar} ${styles.avatarLarge}`}>{initials(enquiry.sender_name)}</span>
          <div>
            <h2 id="enq-title" className={styles.personName}>
              {enquiry.sender_name ?? 'Unknown sender'}
            </h2>
            <p className={styles.personMeta}>
              <Pill tone="estate">{CHANNEL_LABEL[enquiry.channel]}</Pill>
              {contact} · {enquiry.ref}
            </p>
          </div>
        </div>
        <div className={styles.row}>
          {enquiry.category && <Pill tone={CATEGORY_TONE[enquiry.category]}>{CATEGORY_LABEL[enquiry.category]}</Pill>}
          {canRoute && <MoveMenu enquiry={enquiry} onMoved={onMoved} />}
        </div>
      </header>

      <div className={styles.message}>
        {enquiry.subject && <p className={styles.subject}>{enquiry.subject}</p>}
        <p className={styles.bubble}>{enquiry.text}</p>
        <span className={styles.muted}>
          {formatTimestamp(enquiry.received_at)}
          {enquiry.language && enquiry.language !== 'en' ? ` · ${enquiry.language}` : ''}
        </span>
      </div>

      <AiUnderstood enquiry={enquiry} canEdit={canRoute} />
      <DraftPanel enquiry={enquiry} canSend={canSend && !closed} />

      <div className={styles.footerActions}>
        {(enquiry.ai_status === 'FALLBACK' || enquiry.ai_status === 'FAILED') && (
          <Button variant="quiet" size="sm" loading={retry.isPending} onClick={() => retry.mutate()}>
            Ask the AI again
          </Button>
        )}
        {closed ? (
          <Button variant="quiet" size="sm" loading={setStatus.isPending} onClick={() => setStatus.mutate('IN_PROGRESS')}>
            Reopen
          </Button>
        ) : (
          <>
            <Button variant="quiet" size="sm" loading={setStatus.isPending && setStatus.variables === 'CLOSED'} onClick={() => setStatus.mutate('CLOSED')}>
              Close
            </Button>
            <Button variant="quiet" size="sm" loading={setStatus.isPending && setStatus.variables === 'SPAM'} onClick={() => setStatus.mutate('SPAM')}>
              Mark as spam
            </Button>
          </>
        )}
      </div>

      <History events={enquiry.events} />
    </>
  );
}

const EVENT_TEXT: Record<EnquiryEvent['type'], (e: EnquiryEvent) => string> = {
  RECEIVED: () => 'Received',
  CLASSIFIED: (e) => `Read by ${e.actor === 'AI' ? 'the AI' : 'keyword rules'}: ${String(e.to?.category ?? '').toLowerCase()}, sent to ${LANE_LABEL[e.to?.lane as keyof typeof LANE_LABEL] ?? e.to?.lane}`,
  REROUTED: (e) => `Moved from ${LANE_LABEL[e.from?.lane as keyof typeof LANE_LABEL] ?? e.from?.lane} to ${LANE_LABEL[e.to?.lane as keyof typeof LANE_LABEL] ?? e.to?.lane}`,
  CATEGORY_CHANGED: (e) => `Type corrected to ${String(e.to?.category ?? '').toLowerCase()}`,
  FIELDS_CORRECTED: () => 'Details corrected',
  ASSIGNED: () => 'Assigned',
  DRAFT_CREATED: (e) => `Draft v${e.to?.version ?? ''} written${e.to?.source === 'TEMPLATE' ? ' from a template' : ''}`,
  DRAFT_EDITED: (e) => `Draft v${e.to?.version ?? ''} edited`,
  SENT: (e) => `Reply sent by ${String(e.to?.channel ?? '').toLowerCase()}`,
  ALERTED: () => 'GM alerted',
  CLOSED: (e) => `Marked ${String(e.to?.status ?? 'closed').toLowerCase().replace('_', ' ')}`,
};

function History({ events }: { events: readonly EnquiryEvent[] }) {
  return (
    <details className={styles.history}>
      <summary>History · {events.length} events</summary>
      <ol>
        {events.map((e, i) => (
          <li key={`${e.type}-${e.created_at}-${i}`}>
            <span className={styles.historyTime}>{formatTimestamp(e.created_at)}</span>
            <span>
              {EVENT_TEXT[e.type](e)}
              {e.actor_name ? ` · ${e.actor_name}` : ''}
            </span>
          </li>
        ))}
      </ol>
    </details>
  );
}
