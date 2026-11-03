import type { Metadata } from 'next';

import { StaffBookingsPage } from '@/features/staff-bookings';

export const metadata: Metadata = { title: 'Bookings' };

export default function Page() {
  return <StaffBookingsPage />;
}
