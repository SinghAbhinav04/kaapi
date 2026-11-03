import type { Metadata } from 'next';

import { AnalyticsPage } from '@/features/analytics';

export const metadata: Metadata = { title: 'Analytics' };

export default function Page() {
  return <AnalyticsPage />;
}
