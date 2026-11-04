'use client';

import { usePathname } from 'next/navigation';
import { useEffect, useEffectEvent, useState } from 'react';

import { useLogout, useMe, useRequestOtp, useVerifyOtp } from '@/apis/auth';
import { ROUTES } from '@/shared/const';
import { Button, ButtonLink, TextField } from '@/shared/ui';

import { EMAIL_COPY, OTP_LENGTH } from '../const';
import styles from './CheckoutPage.module.css';

/**
 * The guest's email, verified with a one-time code before they can pay: the confirmation,
 * reminders and any refund go here, so a typo has to be caught now. Verifying signs them in
 * as a guest, which is also what lets them see the booking later under "My booking".
 */
export function EmailVerify({ email, onEmail, error }: { email: string; onEmail: (email: string) => void; error?: string }) {
  const me = useMe().data;
  // Signed in already (a guest by code, or staff by password and code): the email is proven.
  const verified = me ? me.email : null;
  const pathname = usePathname();
  const request = useRequestOtp();
  const verify = useVerifyOtp();
  const logout = useLogout();
  const [sentTo, setSentTo] = useState<string | null>(null);
  const [code, setCode] = useState('');

  const sync = useEffectEvent((v: string) => {
    if (v !== email) onEmail(v);
  });
  useEffect(() => {
    if (verified) sync(verified);
  }, [verified]);

  if (verified) {
    return (
      <div className={styles.verify}>
        <TextField label="Email" value={verified} readOnly hint={EMAIL_COPY.verified} />
        <Button
          variant="quiet"
          onClick={() => {
            logout.mutate();
            setSentTo(null);
            setCode('');
            onEmail('');
          }}
        >
          Use a different email
        </Button>
      </div>
    );
  }

  const send = () => request.mutate(email.trim(), { onSuccess: () => setSentTo(email.trim().toLowerCase()) });
  const confirm = () => sentTo && verify.mutate({ email: sentTo, code: code.trim() });

  return (
    <div className={styles.verify}>
      <div className={styles.verifyRow}>
        <TextField
          label="Email"
          type="email"
          autoComplete="email"
          inputMode="email"
          value={email}
          onChange={(e) => {
            onEmail(e.target.value);
            setSentTo(null);
          }}
          hint={sentTo ? undefined : EMAIL_COPY.before}
          error={error ?? request.error?.fieldErrors.email ?? (request.isError ? request.error.message : undefined)}
          required
        />
        <Button variant={sentTo ? 'quiet' : 'ink'} onClick={send} loading={request.isPending} disabled={!email.includes('@')}>
          {sentTo ? 'Send again' : 'Send code'}
        </Button>
      </div>
      {sentTo && (
        <div className={styles.verifyRow}>
          <TextField
            label={`Code sent to ${sentTo}`}
            inputMode="numeric"
            autoComplete="one-time-code"
            maxLength={OTP_LENGTH}
            value={code}
            onChange={(e) => setCode(e.target.value.replace(/\D/g, ''))}
            hint={EMAIL_COPY.after}
            error={verify.isError ? verify.error.message : undefined}
          />
          <Button onClick={confirm} loading={verify.isPending} disabled={code.length !== OTP_LENGTH}>
            Verify
          </Button>
        </div>
      )}
      {verify.error?.code === 'FORBIDDEN' && (
        <ButtonLink href={`${ROUTES.staff.login}?next=${encodeURIComponent(pathname)}`} variant="ink">
          Sign in with your staff password
        </ButtonLink>
      )}
    </div>
  );
}
