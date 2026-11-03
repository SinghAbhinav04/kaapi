'use client';

import { useRouter, useSearchParams } from 'next/navigation';
import { useState, type FormEvent } from 'react';

import { useStaffLogin } from '@/apis/auth';
import { Button, Logo, TextField } from '@/shared/ui';

import { safeNext } from '../lib';
import styles from './StaffLoginPage.module.css';

export function StaffLoginPage() {
  const router = useRouter();
  const params = useSearchParams();
  const login = useStaffLogin();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');

  const submit = (e: FormEvent) => {
    e.preventDefault();
    login.mutate({ email: email.trim(), password }, { onSuccess: () => router.replace(safeNext(params.get('next'))) });
  };

  return (
    <main className={styles.page}>
      <form className={styles.card} onSubmit={submit}>
        <Logo href="/" />
        <div>
          <h1 className={styles.title}>Staff sign-in</h1>
          <p className={styles.lead}>Reservations, sales and management.</p>
        </div>
        <TextField label="Work email" type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} required />
        <TextField label="Password" type="password" autoComplete="current-password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        {login.isError && (
          <p className={styles.error} role="alert">
            {login.error.status === 401 ? 'That email and password don’t match.' : login.error.message}
          </p>
        )}
        <Button type="submit" size="lg" block loading={login.isPending || login.isSuccess}>
          Sign in
        </Button>
        <p className={styles.fine}>Demo accounts are listed in deployment.md.</p>
      </form>
    </main>
  );
}
