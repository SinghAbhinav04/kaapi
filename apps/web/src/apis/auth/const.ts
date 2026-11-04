export const AUTH_PATHS = {
  staffLogin: '/v1/auth/staff/login',
  staffVerify: '/v1/auth/staff/verify',
  otpRequest: '/v1/auth/otp/request',
  otpVerify: '/v1/auth/otp/verify',
  logout: '/v1/auth/logout',
  me: '/v1/me',
} as const;

export const authKeys = {
  all: ['auth'] as const,
  me: () => [...authKeys.all, 'me'] as const,
};

export const AUTH_STALE_MS = 5 * 60_000;
