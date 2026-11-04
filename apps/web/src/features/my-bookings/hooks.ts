'use client';

import { useState } from 'react';

import { useRequestOtp, useVerifyOtp } from '@/apis/auth';

/** Email → one-time code → signed in. The code arrives by email only. */
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
