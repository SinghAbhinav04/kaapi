import type { Metadata } from 'next';

import { DashboardPage } from '@/features/operations';

export const metadata: Metadata = { title: 'Today' };

export default function Page() {
  return <DashboardPage />;
}
