import { http, type HttpClient } from '@/apis/http';

import { AUTH_PATHS } from './const';
import type { AuthApi } from './interface';

export function createAuthApi(client: HttpClient): AuthApi {
  return {
    staffLogin: (email, password) => client.post(AUTH_PATHS.staffLogin, { email, password }),
    staffVerify: (email, code) => client.post(AUTH_PATHS.staffVerify, { email, code }),
    requestOtp: (email) => client.post(AUTH_PATHS.otpRequest, { email }),
    verifyOtp: (email, code) => client.post(AUTH_PATHS.otpVerify, { email, code }),
    me: (signal) => client.get(AUTH_PATHS.me, { signal }),
    logout: () => client.post(AUTH_PATHS.logout),
  };
}

export const authApi = createAuthApi(http);
