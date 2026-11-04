import { ROUTES } from '@/shared/const';

/** Only same-site staff paths, or a checkout in progress, are allowed as a return target (no open redirects). */
export function safeNext(next: string | null): string {
  const allowed = next && (next.startsWith('/staff/') || next.startsWith('/book/')) && !next.startsWith('//') && !next.includes('\\');
  return allowed ? next : ROUTES.staff.inbox;
}
