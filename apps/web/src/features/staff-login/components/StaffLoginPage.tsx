'use client';

import { useRouter, useSearchParams } from 'next/navigation';
import { useState, type FormEvent } from 'react';

import { useStaffLogin, useStaffVerify } from '@/apis/auth';
import { Button, Logo, TextField } from '@/shared/ui';

import { safeNext } from '../lib';
import styles from './StaffLoginPage.module.css';

const CODE_LENGTH = 6;

/** Two steps: the password, then a code emailed to the staff member. */
export function StaffLoginPage() {
  const router = useRouter();
  const params = useSearchParams();
  const login = useStaffLogin();
  const verify = useStaffVerify();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [code, setCode] = useState('');
  const challenge = login.data;

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (!challenge) {
      login.mutate({ email: email.trim(), password });
      return;
    }
    verify.mutate({ email: email.trim(), code: code.trim() }, { onSuccess: () => router.replace(safeNext(params.get('next'))) });
  };

  const error = login.isError
    ? login.error.status === 401
      ? 'That email and password don’t match.'
      : login.error.message
    : verify.isError
      ? verify.error.message
      : null;

  return (
    <main className={styles.page}>
      <form className={styles.card} onSubmit={submit}>
        <Logo href="/" />
        <div>
          <h1 className={styles.title}>Staff sign-in</h1>
          <p className={styles.lead}>{challenge ? 'Enter the code we just emailed you.' : 'Reservations, sales and management.'}</p>
        </div>
        {!challenge ? (
          <>
            <TextField label="Work email" type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} required />
            <TextField
              label="Password"
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </>
        ) : (
          <TextField
            label={`Code sent to ${challenge.sent_to}`}
            inputMode="numeric"
            autoComplete="one-time-code"
            maxLength={CODE_LENGTH}
            value={code}
            onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
            hint="Check your work inbox. The code works for 5 minutes."
            required
          />
        )}
        {error && (
          <p className={styles.error} role="alert">
            {error}
          </p>
        )}
        <Button
          type="submit"
          size="lg"
          block
          loading={login.isPending || verify.isPending || verify.isSuccess}
          disabled={!!challenge && code.length !== CODE_LENGTH}
        >
          {challenge ? 'Verify and sign in' : 'Continue'}
        </Button>
        {challenge && (
          <Button variant="quiet" block onClick={() => login.reset()}>
            Use a different account
          </Button>
        )}
        <p className={styles.fine}>Demo accounts are listed in deployment.md.</p>
      </form>
    </main>
  );
}
