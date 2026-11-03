'use client';

import { useState } from 'react';

import { useEditDraft, useSendReply, type Enquiry } from '@/apis/enquiries';
import { formatTimestamp } from '@/shared/lib/dates';
import { Button, Pill, TextAreaField } from '@/shared/ui';

import { SEND_LABEL } from '../const';
import { parseDraft } from '../lib';
import styles from './Inbox.module.css';

export function DraftPanel({ enquiry, canSend }: { enquiry: Enquiry; canSend: boolean }) {
  const draft = enquiry.draft;
  const [editing, setEditing] = useState(false);
  const [text, setText] = useState('');
  const save = useEditDraft(enquiry.ref);
  const send = useSendReply(enquiry.ref);

  const sent = !!draft?.sent_at;
  // Staff can always write their own reply: from scratch when there's no draft (or the AI is
  // still reading), as a follow-up after a send, or by editing the current draft.
  const startEdit = () => {
    setText(draft && !sent ? draft.body : '');
    setEditing(true);
  };

  const editor = (
    <div className={styles.editor}>
      <TextAreaField label={draft && !sent ? 'Edit the reply' : 'Write your reply'} value={text} onChange={(e) => setText(e.target.value)} rows={12} />
      {save.isError && (
        <p className={styles.error} role="alert">
          {save.error.message}
        </p>
      )}
      <div className={styles.row}>
        <Button variant="quiet" onClick={() => setEditing(false)}>
          Cancel
        </Button>
        <Button loading={save.isPending} onClick={() => save.mutate(text, { onSuccess: () => setEditing(false) })} disabled={!text.trim()}>
          Save draft
        </Button>
      </div>
    </div>
  );

  if (!draft) {
    return (
      <section className={styles.block}>
        <h3 className={styles.blockTitle}>Reply draft</h3>
        {editing ? (
          editor
        ) : (
          <div className={styles.sendRow}>
            <span className={styles.muted}>{enquiry.ai_status === 'PENDING' ? 'The AI is still reading this message. You can write the reply yourself.' : 'No draft for this enquiry yet.'}</span>
            <Button variant="quiet" onClick={startEdit}>
              Write a reply
            </Button>
          </div>
        )}
      </section>
    );
  }

  return (
    <section aria-labelledby="draft-title" className={styles.block}>
      <div className={styles.blockHead}>
        <h3 id="draft-title" className={styles.blockTitle}>
          Reply draft
        </h3>
        <span className={styles.muted}>
          v{draft.version} · {draft.author === 'USER' ? 'edited by staff' : 'AI draft'}
        </span>
      </div>

      {draft.flagged_reason && !sent && (
        <p className={styles.flag} role="note">
          <strong>Checked by the system:</strong> {draft.flagged_reason}.
        </p>
      )}

      {editing ? (
        editor
      ) : (
        <div className={styles.draft}>
          {parseDraft(draft.body).map((block, i) =>
            block.kind === 'text' ? (
              <p key={i} className={styles.draftText}>
                {block.text}
              </p>
            ) : (
              <div key={i} className={styles.quote}>
                <p className={styles.quoteLabel}>
                  <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" aria-hidden="true">
                    <rect x="5" y="11" width="14" height="10" rx="2" />
                    <path d="M8 11V7a4 4 0 018 0v4" />
                  </svg>
                  Live from the rate table · not written by AI
                </p>
                {block.rows.map((r) => (
                  <div key={r.room} className={styles.quoteRow}>
                    <span className={styles.quoteRoom}>{r.room}</span>
                    {r.left !== null && <Pill tone="cherry">{r.left} left</Pill>}
                    <span className={styles.quotePrice}>
                      <span className="mono">{r.roomOnly}</span> room only
                      {r.breakfast && (
                        <>
                          {' · '}
                          <span className="mono">{r.breakfast}</span> with breakfast
                        </>
                      )}
                    </span>
                  </div>
                ))}
                {block.head && <p className={styles.quoteFoot}>{block.head.replace(/^For /, '').replace(/, totals including GST:$/, '')} · totals include GST</p>}
              </div>
            ),
          )}
        </div>
      )}

      {sent ? (
        !editing && (
          <div className={styles.sendRow}>
            <p className={styles.sent} role="status">
              Sent {formatTimestamp(draft.sent_at!)}. A copy is in the Outbox.
            </p>
            <Button variant="quiet" onClick={startEdit}>
              Write a follow-up
            </Button>
          </div>
        )
      ) : (
        !editing && (
          <div className={styles.sendRow}>
            <span className={styles.muted}>Nothing is sent until you press Send.</span>
            <div className={styles.row}>
              <Button variant="quiet" onClick={startEdit}>
                Edit
              </Button>
              {canSend && (
                <Button onClick={() => send.mutate()} loading={send.isPending}>
                  {SEND_LABEL[enquiry.channel]}
                </Button>
              )}
            </div>
            {send.isError && (
              <p className={styles.error} role="alert">
                {send.error.message}
              </p>
            )}
          </div>
        )
      )}
    </section>
  );
}
