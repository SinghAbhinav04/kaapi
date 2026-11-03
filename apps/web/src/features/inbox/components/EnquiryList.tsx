'use client';

import type { EnquirySummary, Lane } from '@/apis/enquiries';
import { formatAge } from '@/shared/lib/dates';
import { cx } from '@/shared/lib/cx';
import { initials } from '@/shared/lib/text';
import { EmptyState, ErrorState, Pill, Skeleton } from '@/shared/ui';

import { CHANNEL_LABEL, LANE_LABEL } from '../const';
import { complaintSummary, itemBadge } from '../lib';
import { ChannelIcon } from './ChannelIcon';
import styles from './Inbox.module.css';

interface EnquiryListProps {
  lane: Lane | null;
  list: readonly EnquirySummary[];
  all: readonly EnquirySummary[];
  open: string | null;
  search: string;
  onSearch: (q: string) => void;
  onOpen: (ref: string) => void;
  onLane: (lane: Lane) => void;
  loading: boolean;
  stale: boolean;
  error: { message: string; retry: () => void } | null;
  onPaste: () => void;
}

export function EnquiryList({ lane, list, all, open, search, onSearch, onOpen, onLane, loading, stale, error, onPaste }: EnquiryListProps) {
  const complaints = complaintSummary(all);
  return (
    <section aria-labelledby="lane-title" className={styles.listPane}>
      <div className={styles.listHead}>
        <h1 id="lane-title" className={styles.laneTitle}>
          {lane ? LANE_LABEL[lane] : 'All enquiries'}
        </h1>
        <Pill tone="estate">{list.length} open</Pill>
      </div>
      <label className={styles.search}>
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="var(--grey)" strokeWidth="2" strokeLinecap="round" aria-hidden="true">
          <circle cx="11" cy="11" r="7" />
          <path d="M20 20l-3.5-3.5" />
        </svg>
        <span className="sr-only">Search enquiries</span>
        <input type="search" placeholder="Search guests, refs, messages" value={search} onChange={(e) => onSearch(e.target.value)} />
      </label>

      {complaints.count > 0 && lane !== 'GM' && (
        <button type="button" className={styles.alert} onClick={() => onLane('GM')}>
          <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" aria-hidden="true">
            <path d="M12 9v4M12 17h.01" />
            <path d="M10.3 3.9L1.8 18a2 2 0 001.7 3h17a2 2 0 001.7-3L13.7 3.9a2 2 0 00-3.4 0z" />
          </svg>
          <span className={styles.alertText}>
            <strong>
              {complaints.count} complaint{complaints.count > 1 ? 's' : ''} need{complaints.count > 1 ? '' : 's'} the GM
            </strong>
            {complaints.oldest && `Oldest waiting ${formatAge(complaints.oldest)}`}
          </span>
          <span className={styles.alertGo}>Open</span>
        </button>
      )}

      <button type="button" className={styles.paste} onClick={onPaste}>
        + Paste an email or WhatsApp message
      </button>

      {error ? (
        <ErrorState compact message={error.message} onRetry={error.retry} />
      ) : loading ? (
        <div className={styles.skeletons}>
          {Array.from({ length: 6 }, (_, i) => (
            <Skeleton key={i} height={72} radius={14} />
          ))}
        </div>
      ) : list.length === 0 ? (
        <EmptyState compact title={search ? 'Nothing matches that search' : 'All caught up'}>
          {search ? 'Try a name, a reference like ENQ-4411, or a word from the message.' : 'New messages appear here within seconds.'}
        </EmptyState>
      ) : (
        <ul className={cx(styles.items, stale && styles.stale)}>
          {list.map((e) => (
            <li key={e.ref}>
              <ListItem enquiry={e} active={e.ref === open} onOpen={onOpen} />
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}

function ListItem({ enquiry: e, active, onOpen }: { enquiry: EnquirySummary; active: boolean; onOpen: (ref: string) => void }) {
  const badge = itemBadge(e);
  const urgent = e.category === 'COMPLAINT' && e.status === 'NEW';
  return (
    <button type="button" className={cx(styles.item, active && styles.itemActive)} aria-current={active || undefined} onClick={() => onOpen(e.ref)}>
      <span className={cx(styles.avatar, urgent && styles.avatarAlert, active && styles.avatarActive)}>
        {initials(e.sender_name)}
        <span className={styles.channel} title={CHANNEL_LABEL[e.channel]}>
          <ChannelIcon channel={e.channel} />
        </span>
      </span>
      <span className={styles.itemBody}>
        <span className={styles.itemTop}>
          <strong>{e.sender_name ?? 'Unknown sender'}</strong>
          <span className={urgent ? styles.ageUrgent : styles.age}>{formatAge(e.received_at)}</span>
        </span>
        <span className={styles.snippet}>{e.snippet}</span>
        {badge && (
          <Pill tone={badge.tone} className={styles.badge}>
            {badge.label}
          </Pill>
        )}
      </span>
    </button>
  );
}
