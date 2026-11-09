import type { AuthResponse, StaffChallenge } from './types';

/** True when step 1 already signed the staff member in (a password-only account). */
export const isSignedIn = (r: StaffChallenge | AuthResponse): r is AuthResponse => 'user' in r;
