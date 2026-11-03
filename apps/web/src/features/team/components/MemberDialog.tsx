'use client';

import { useEffect, useRef, useState, type FormEvent } from 'react';

import { useCreateMember, type AccessRole } from '@/apis/access';
import { Button, SelectField, TextField } from '@/shared/ui';

import { COPY, MIN_PASSWORD } from '../const';
import { generatePassword } from '../lib';
import styles from './Team.module.css';

/** Add a staff account with a role and a starting password. */
export function MemberDialog({ open, roles, onClose }: { open: boolean; roles: AccessRole[]; onClose: () => void }) {
  const dialog = useRef<HTMLDialogElement>(null);
  const [name, setName] = useState('');
  const [email, setEmail] = useState('');
  const [phone, setPhone] = useState('');
  const [roleId, setRoleId] = useState('');
  const [password, setPassword] = useState('');
  const create = useCreateMember();
  const role = roleId || roles.find((r) => !r.system)?.id || roles[0]?.id || '';

  useEffect(() => {
    const d = dialog.current;
    if (!d) return;
    if (open && !d.open) d.showModal();
    if (!open && d.open) d.close();
  }, [open]);

  const submit = (e: FormEvent) => {
    e.preventDefault();
    create.mutate(
      { name, email, phone: phone || undefined, role_id: role, password },
      {
        onSuccess: () => {
          setName('');
          setEmail('');
          setPhone('');
          setPassword('');
          onClose();
        },
      },
    );
  };
  const field = (k: string) => create.error?.fieldErrors[k];

  return (
    <dialog ref={dialog} className={styles.dialog} onClose={onClose} aria-labelledby="member-title">
      <form className={styles.dialogBody} onSubmit={submit}>
        <h2 id="member-title" className={styles.dialogTitle}>
          Add person
        </h2>
        <TextField label="Name" value={name} onChange={(e) => setName(e.target.value)} error={field('name')} required />
        <TextField label="Work email" type="email" value={email} onChange={(e) => setEmail(e.target.value)} error={field('email')} required />
        <TextField label="Phone" optional type="tel" value={phone} onChange={(e) => setPhone(e.target.value)} />
        <SelectField
          label="Role"
          value={role}
          onChange={(e) => setRoleId(e.target.value)}
          options={roles.map((r) => ({ value: r.id, label: r.name }))}
          hint={roles.find((r) => r.id === role)?.description}
        />
        <div className={styles.passwordRow}>
          <TextField
            label="Starting password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            minLength={MIN_PASSWORD}
            hint={COPY.passwordHint}
            error={field('password')}
            autoComplete="new-password"
            required
          />
          <Button variant="quiet" onClick={() => setPassword(generatePassword())}>
            Generate
          </Button>
        </div>
        {create.isError && Object.keys(create.error.fieldErrors).length === 0 && (
          <p className={styles.error} role="alert">
            {create.error.message}
          </p>
        )}
        <div className={styles.row}>
          <Button variant="quiet" onClick={onClose}>
            Cancel
          </Button>
          <Button type="submit" loading={create.isPending} disabled={!name.trim() || !email.trim() || password.length < MIN_PASSWORD || !role}>
            Add person
          </Button>
        </div>
      </form>
    </dialog>
  );
}
