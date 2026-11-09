'use client';

import { useRouter, useSearchParams } from 'next/navigation';
import { useState, type FormEvent } from 'react';

import { isSignedIn, useStaffLogin, useStaffVerify } from '@/apis/auth';
import { Button, Logo, TextField } from '@/shared/ui';

import { CODE_LENGTH, COPY } from '../const';
import { safeNext } from '../lib';
import styles from './StaffLoginPage.module.css';

/** Two steps: the password, then a code emailed to the staff member (password-only accounts skip the code). */
export function StaffLoginPage() {
  const router = useRouter();
  const params = useSearchParams();
  const login = useStaffLogin();
  const verify = useStaffVerify();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [code, setCode] = useState('');
  const challenge = login.data && !isSignedIn(login.data) ? login.data : undefined;
  const done = () => router.replace(safeNext(params.get('next')));

  const submit = (e: FormEvent) => {
    e.preventDefault();
    if (!challenge) {
      login.mutate({ email: email.trim(), password }, { onSuccess: (data) => isSignedIn(data) && done() });
      return;
    }
    verify.mutate({ email: email.trim(), code: code.trim() }, { onSuccess: done });
  };

  const error = login.isError
    ? login.error.status === 401
      ? COPY.wrongPassword
      : login.error.message
    : verify.isError
      ? verify.error.message
      : null;

  return (
    <main className={styles.page}>
      <form className={styles.card} onSubmit={submit}>
        <Logo href="/" />
        <div>
          <h1 className={styles.title}>{COPY.title}</h1>
          <p className={styles.lead}>{challenge ? COPY.leadCode : COPY.leadPassword}</p>
        </div>
        {!challenge ? (
          <>
            <TextField label={COPY.email} type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} required />
            <TextField
              label={COPY.password}
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </>
        ) : (
          <TextField
            label={COPY.codeLabel(challenge.sent_to)}
            inputMode="numeric"
            autoComplete="one-time-code"
            maxLength={CODE_LENGTH}
            value={code}
            onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
            hint={COPY.codeHint}
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
          loading={login.isPending || (login.isSuccess && !challenge) || verify.isPending || verify.isSuccess}
          disabled={!!challenge && code.length !== CODE_LENGTH}
        >
          {challenge ? COPY.verify : COPY.continue}
        </Button>
        {challenge && (
          <Button variant="quiet" block onClick={() => login.reset()}>
            {COPY.otherAccount}
          </Button>
        )}
        <p className={styles.fine}>{COPY.demo}</p>
      </form>
    </main>
  );
}
