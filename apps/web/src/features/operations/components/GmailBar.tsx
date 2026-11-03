'use client';

import { useSearchParams } from 'next/navigation';

import { useCan } from '@/apis/auth';
import { useConnectGmail, useDisconnectGmail, useGmailStatus, useSyncGmail } from '@/apis/operations';
import { formatAge } from '@/shared/lib/dates';
import { Button, Pill } from '@/shared/ui';

import { GMAIL_COPY } from '../const';
import styles from './Mail.module.css';

/**
 * The connected mailbox, above the inbox: who we read and reply as, when it last synced,
 * and (for the GM and admin) connect, sync now and disconnect.
 */
export function GmailBar({ onCompose }: { onCompose: () => void }) {
  const status = useGmailStatus();
  const canManage = useCan('integrations:manage');
  const canSend = useCan('enquiries:send');
  const connect = useConnectGmail();
  const sync = useSyncGmail();
  const disconnect = useDisconnectGmail();
  const params = useSearchParams();
  const returned = params.get('gmail');
  const reason = params.get('reason');

  const g = status.data?.gmail;
  const error = connect.error ?? sync.error ?? disconnect.error;

  return (
    <section className={styles.bar} aria-label="Gmail">
      <div className={styles.barMain}>
        <Pill tone={g?.connected ? 'estate' : 'neutral'}>{g?.connected ? 'Gmail connected' : 'Gmail off'}</Pill>
        <p className={styles.barText}>
          {!g
            ? 'Checking the mailbox…'
            : g.connected && g.account_email
              ? `${GMAIL_COPY.connected(g.account_email)}${g.last_sync_at ? ` · synced ${formatAge(g.last_sync_at)}` : ''}`
              : !g.configured
                ? GMAIL_COPY.notConfigured
                : canManage
                  ? GMAIL_COPY.disconnected
                  : GMAIL_COPY.disconnectedStaff}
        </p>
      </div>
      <div className={styles.barActions}>
        {canSend && (
          <Button variant="ink" onClick={onCompose}>
            New message
          </Button>
        )}
        {canManage && g?.connected && (
          <>
            <Button variant="quiet" onClick={() => sync.mutate()} loading={sync.isPending}>
              Sync now
            </Button>
            <Button variant="quiet" onClick={() => disconnect.mutate()} loading={disconnect.isPending}>
              Disconnect
            </Button>
          </>
        )}
        {canManage && g && !g.connected && g.configured && (
          <Button onClick={() => connect.mutate()} loading={connect.isPending}>
            Connect Gmail
          </Button>
        )}
      </div>
      {returned && GMAIL_COPY.returned[returned] && (
        <p className={returned === 'error' ? styles.barError : styles.barNote} role="status">
          {GMAIL_COPY.returned[returned]}
          {reason ? ` ${reason}` : ''}
        </p>
      )}
      {g?.connected && g.last_error && (
        <p className={styles.barError} role="alert">
          Last sync failed: {g.last_error}
        </p>
      )}
      {error && (
        <p className={styles.barError} role="alert">
          {error.message}
        </p>
      )}
    </section>
  );
}
