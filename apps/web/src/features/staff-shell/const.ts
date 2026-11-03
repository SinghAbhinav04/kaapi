import type { Lane } from '@/apis/enquiries';
import type { Permission } from '@/apis/auth';
import { ROUTES } from '@/shared/const';

export type NavIcon = 'inbox' | 'rates' | 'bookings' | 'outbox' | 'dashboard' | 'analytics' | 'offers' | 'team';

export const STAFF_NAV: readonly { href: string; label: string; icon: NavIcon; permission: Permission }[] = [
  { href: ROUTES.staff.inbox, label: 'Inbox', icon: 'inbox', permission: 'enquiries:read' },
  { href: ROUTES.staff.rates, label: 'Rates & allotment', icon: 'rates', permission: 'rates:read' },
  { href: ROUTES.staff.bookings, label: 'Bookings', icon: 'bookings', permission: 'bookings:read' },
  { href: ROUTES.staff.outbox, label: 'Outbox', icon: 'outbox', permission: 'outbox:read' },
  { href: ROUTES.staff.dashboard, label: 'Dashboard', icon: 'dashboard', permission: 'dashboard:read' },
  { href: ROUTES.staff.analytics, label: 'Analytics', icon: 'analytics', permission: 'analytics:read' },
  { href: ROUTES.staff.offers, label: 'Offers', icon: 'offers', permission: 'promos:write' },
  { href: ROUTES.staff.team, label: 'Team & access', icon: 'team', permission: 'users:manage' },
];

/** Lane order and colours in the sidebar (from the Inbox board). */
export const LANES: readonly { lane: Lane; label: string; dot: 'cherry' | 'dashed' | 'estate' | 'gold' }[] = [
  { lane: 'GM', label: 'Complaints & GM', dot: 'cherry' },
  { lane: 'UNSORTED', label: 'Unsorted', dot: 'dashed' },
  { lane: 'RESERVATIONS', label: 'Reservations', dot: 'estate' },
  { lane: 'SALES', label: 'Sales', dot: 'gold' },
];

export const ROLE_TITLE: Record<string, string> = {
  GM: 'General Manager',
  RESERVATIONS: 'Reservations',
  SALES: 'Events & Sales',
  ADMIN: 'Administrator',
  GUEST: 'Guest',
};
