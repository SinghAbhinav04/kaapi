import type { Metadata } from 'next';

import { MyBookingsPage } from '@/features/my-bookings';

export const metadata: Metadata = { title: 'My bookings', robots: { index: false } };

export default function Page() {
  return <MyBookingsPage />;
}
