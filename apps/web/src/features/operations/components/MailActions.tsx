'use client';

import { useCan } from '@/apis/auth';
import { useGmailStatus } from '@/apis/operations';
import { Button } from '@/shared/ui';

import styles from './Mail.module.css';

/**
 * Above the inbox: "New message". The mailbox is connected by the server itself (see
 * cmd/gmail-auth), so there's nothing to set up here; the only thing shown about it is a
 * sync failure, because that means mail isn't arriving.
 */
export function MailActions({ onCompose }: { onCompose: () => void }) {
  const canSend = useCan('enquiries:send');
  const failure = useGmailStatus().data?.gmail.last_error;
  if (!canSend && !failure) return null;
  return (
    <div className={styles.actions}>
      {failure && (
        <p className={styles.failure} role="alert">
          Email isn&rsquo;t syncing: {failure}
        </p>
      )}
      {canSend && (
        <Button variant="ink" onClick={onCompose}>
          New message
        </Button>
      )}
    </div>
  );
}
