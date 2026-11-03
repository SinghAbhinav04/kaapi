'use client';

import { useEffect, useRef, useState, type FormEvent } from 'react';

import { useInboundMessage } from '@/apis/enquiries';
import { Button, Segmented, TextAreaField, TextField } from '@/shared/ui';

import styles from './Inbox.module.css';

type InboundChannel = 'EMAIL' | 'WHATSAPP';

const CHANNELS = [
  { value: 'EMAIL', label: 'Email' },
  { value: 'WHATSAPP', label: 'WhatsApp' },
] as const;

/**
 * Email and WhatsApp are simulated in this build: paste a message and it enters the same
 * pipeline a provider webhook would feed (AI read, routing, draft).
 */
export function InboundDialog({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: (ref: string) => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [channel, setChannel] = useState<InboundChannel>('EMAIL');
  const [name, setName] = useState('');
  const [contact, setContact] = useState('');
  const [subject, setSubject] = useState('');
  const [text, setText] = useState('');
  const inbound = useInboundMessage();

  useEffect(() => {
    const d = dialog.current;
    if (!d) return;
    if (open && !d.open) d.showModal();
    if (!open && d.open) d.close();
  }, [open]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    inbound.mutate(
      {
        channel,
        sender_name: name.trim() || undefined,
        sender_email: channel === 'EMAIL' ? contact.trim() : undefined,
        sender_phone: channel === 'WHATSAPP' ? contact.trim() : undefined,
        subject: channel === 'EMAIL' ? subject.trim() || undefined : undefined,
        text,
      },
      {
        onSuccess: ({ ref }) => {
          setName('');
          setContact('');
          setSubject('');
          setText('');
          onCreated(ref);
        },
      },
    );
  };

  return (
    <dialog ref={dialog} className={styles.dialog} onClose={onClose} aria-labelledby="inbound-title">
      <form className={styles.dialogBody} onSubmit={submit}>
        <h2 id="inbound-title" className={styles.blockTitle}>
          Paste a message
        </h2>
        <p className={styles.muted}>In production these arrive from the email and WhatsApp providers. Here you can paste one in.</p>
        <Segmented label="Channel" options={CHANNELS} value={channel} onChange={setChannel} />
        <TextField label="Sender name" value={name} onChange={(e) => setName(e.target.value)} />
        <TextField
          label={channel === 'EMAIL' ? 'Sender email' : 'Sender WhatsApp number'}
          type={channel === 'EMAIL' ? 'email' : 'tel'}
          value={contact}
          onChange={(e) => setContact(e.target.value)}
          required
        />
        {channel === 'EMAIL' && <TextField label="Subject" optional value={subject} onChange={(e) => setSubject(e.target.value)} />}
        <TextAreaField label="Message" rows={5} value={text} onChange={(e) => setText(e.target.value)} required />
        {inbound.isError && (
          <p className={styles.error} role="alert">
            {Object.values(inbound.error.fieldErrors)[0] ?? inbound.error.message}
          </p>
        )}
        <div className={styles.row}>
          <Button variant="quiet" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" loading={inbound.isPending} disabled={!text.trim() || !contact.trim()}>
            Add to inbox
          </Button>
        </div>
      </form>
    </dialog>
  );
}
