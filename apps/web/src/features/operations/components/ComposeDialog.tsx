'use client';

import { useEffect, useRef, useState, type FormEvent } from 'react';

import { useCompose, useGmailStatus } from '@/apis/operations';
import { Button, TextAreaField, TextField } from '@/shared/ui';

import styles from './Mail.module.css';

/** A new email written by staff. Goes out through Gmail when connected; otherwise simulated. */
export function ComposeDialog({ open, onClose }: { open: boolean; onClose: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [to, setTo] = useState('');
  const [subject, setSubject] = useState('');
  const [body, setBody] = useState('');
  const compose = useCompose();
  const connected = useGmailStatus().data?.gmail.connected ?? false;

  useEffect(() => {
    const d = dialog.current;
    if (!d) return;
    if (open && !d.open) d.showModal();
    if (!open && d.open) d.close();
  }, [open]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    compose.mutate(
      { to: to.trim(), subject: subject.trim(), body },
      {
        onSuccess: () => {
          setTo('');
          setSubject('');
          setBody('');
          onClose();
        },
      },
    );
  };

  const fieldError = (name: string) => compose.error?.fieldErrors[name];

  return (
    <dialog ref={dialog} className={styles.dialog} onClose={onClose} aria-labelledby="compose-title">
      <form className={styles.dialogBody} onSubmit={submit}>
        <h2 id="compose-title" className={styles.dialogTitle}>
          New message
        </h2>
        <p className={styles.muted}>
          {connected
            ? 'Sends from the connected Gmail mailbox. A copy is kept in the Outbox.'
            : 'No mailbox is connected, so this is recorded in the Outbox but not delivered.'}
        </p>
        <TextField label="To" type="email" value={to} onChange={(e) => setTo(e.target.value)} error={fieldError('to')} required />
        <TextField label="Subject" value={subject} onChange={(e) => setSubject(e.target.value)} error={fieldError('subject')} maxLength={200} required />
        <TextAreaField label="Message" rows={8} value={body} onChange={(e) => setBody(e.target.value)} error={fieldError('body')} maxLength={4000} required />
        {compose.isError && Object.keys(compose.error.fieldErrors).length === 0 && (
          <p className={styles.barError} role="alert">
            {compose.error.message}
          </p>
        )}
        <div className={styles.row}>
          <Button variant="quiet" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" loading={compose.isPending} disabled={!to.trim() || !subject.trim() || !body.trim()}>
            {connected ? 'Send' : 'Record message'}
          </Button>
        </div>
      </form>
    </dialog>
  );
}
