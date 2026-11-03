'use client';

import { useEffect, useRef, useState, type FormEvent } from 'react';

import { useUpdateMember, type Member } from '@/apis/access';
import { Button, TextField } from '@/shared/ui';

import { COPY, MIN_PASSWORD } from '../const';
import { generatePassword } from '../lib';
import styles from './Team.module.css';

/** Give someone a new password; it signs them out everywhere. */
export function PasswordDialog({ member, onClose }: { member: Member | null; onClose: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [password, setPassword] = useState('');
  const update = useUpdateMember();

  useEffect(() => {
    const d = dialog.current;
    if (!d) return;
    if (member && !d.open) d.showModal();
    if (!member && d.open) d.close();
  }, [member]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (!member) return;
    update.mutate(
      { id: member.id, body: { password } },
      {
        onSuccess: () => {
          setPassword('');
          onClose();
        },
      },
    );
  };

  return (
    <dialog ref={dialog} className={styles.dialog} onClose={onClose} aria-labelledby="password-title">
      <form className={styles.dialogBody} onSubmit={submit}>
        <h2 id="password-title" className={styles.dialogTitle}>
          New password for {member?.name}
        </h2>
        <div className={styles.passwordRow}>
          <TextField
            label="New password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            minLength={MIN_PASSWORD}
            hint={COPY.passwordHint}
            error={update.error?.fieldErrors.password}
            autoComplete="new-password"
            required
          />
          <Button variant="quiet" onClick={() => setPassword(generatePassword())}>
            Generate
          </Button>
        </div>
        {update.isError && !update.error.fieldErrors.password && (
          <p className={styles.error} role="alert">
            {update.error.message}
          </p>
        )}
        <div className={styles.row}>
          <Button variant="quiet" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" loading={update.isPending} disabled={password.length < MIN_PASSWORD}>
            Set password
          </Button>
        </div>
      </form>
    </dialog>
  );
}
