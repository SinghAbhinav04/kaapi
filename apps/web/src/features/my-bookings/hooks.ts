'use client';

import { useState } from 'react';

import { useRequestOtp, useVerifyOtp } from '@/apis/auth';

/** Email → one-time code → signed in. The code is sent by email (simulated in the demo). */
export function useOtpSignIn() {
  const [email, setEmail] = useState('');
  const [code, setCode] = useState('');
  const request = useRequestOtp();
  const verify = useVerifyOtp();
  const sent = request.isSuccess;

  return {
    email,
    setEmail,
    code,
    setCode,
    sent,
    /** Shown only when the API runs in demo mode, since no real email is sent. */
    demoCode: request.data?.demo_code,
    sendCode: () => request.mutate(email.trim()),
    verifyCode: () => verify.mutate({ email: email.trim(), code: code.trim() }),
    restart: () => {
      request.reset();
      verify.reset();
      setCode('');
    },
    sending: request.isPending,
    verifying: verify.isPending,
    error: verify.error ?? request.error,
  };
}
