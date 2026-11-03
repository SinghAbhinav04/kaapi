import type { AuthResponse, MeResponse, OtpRequestResponse } from './types';

/**
 * Sign-in for staff (password) and guests (one-time code by email). The web app keeps
 * tokens in httpOnly cookies the API sets; nothing here touches a token directly.
 */
export interface AuthApi {
  staffLogin(email: string, password: string): Promise<AuthResponse>;
  requestOtp(email: string): Promise<OtpRequestResponse>;
  verifyOtp(email: string, code: string): Promise<AuthResponse>;
  me(signal?: AbortSignal): Promise<MeResponse>;
  logout(): Promise<void>;
}
