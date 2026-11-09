import type {
  StaffChallenge, AuthResponse, MeResponse, OtpRequestResponse } from './types';

/**
 * Sign-in for staff (password) and guests (one-time code by email). The web app keeps
 * tokens in httpOnly cookies the API sets; nothing here touches a token directly.
 */
export interface AuthApi {
  /** Step 1: a correct password emails a code, or signs a password-only account in at once. */
  staffLogin(email: string, password: string): Promise<StaffChallenge | AuthResponse>;
  /** Step 2: the emailed code signs the staff member in. */
  staffVerify(email: string, code: string): Promise<AuthResponse>;
  requestOtp(email: string): Promise<OtpRequestResponse>;
  verifyOtp(email: string, code: string): Promise<AuthResponse>;
  me(signal?: AbortSignal): Promise<MeResponse>;
  logout(): Promise<void>;
}
