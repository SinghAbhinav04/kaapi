import { ROUTES } from '@/shared/const';

/** Only same-site staff paths are allowed as a return target (no open redirects). */
export function safeNext(next: string | null): string {
  return next && next.startsWith('/staff/') && !next.startsWith('//') && !next.includes('\\') ? next : ROUTES.staff.inbox;
}
