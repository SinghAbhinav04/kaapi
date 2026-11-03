'use client';

import { useCan } from '@/apis/auth';
import { useSyncMail } from '@/apis/enquiries';
import { useGmailStatus } from '@/apis/operations';
import { Button } from '@/shared/ui';

import styles from './Mail.module.css';

/**
 * Above the inbox: "Sync mail" (imports everything since the last sync, e.g. after the server
 * was down) and "New message". The mailbox is connected by the server itself (see
 * cmd/gmail-auth), so there's nothing to set up here; the only state shown is a sync failure.
 */
export function MailActions({ onCompose }: { onCompose: () => void }) {
  const canSend = useCan('enquiries:send');
  const canSync = useCan('enquiries:read');
  const sync = useSyncMail();
  const failure = useGmailStatus().data?.gmail.last_error;
  if (!canSend && !canSync && !failure) return null;
  return (
    <div className={styles.actions}>
      {failure && (
        <p className={styles.failure} role="alert">
          Email isn&rsquo;t syncing: {failure}
        </p>
      )}
      {sync.isSuccess && (
        <p className={styles.note} role="status">
          {sync.data.imported === 0 ? 'Mail is up to date.' : `${sync.data.imported} new ${sync.data.imported === 1 ? 'message' : 'messages'} imported.`}
        </p>
      )}
      {sync.isError && (
        <p className={styles.failure} role="alert">
          {sync.error.message}
        </p>
      )}
      {canSync && (
        <Button variant="quiet" onClick={() => sync.mutate()} loading={sync.isPending}>
          Sync mail
        </Button>
      )}
      {canSend && (
        <Button variant="ink" onClick={onCompose}>
          New message
        </Button>
      )}
    </div>
  );
}
