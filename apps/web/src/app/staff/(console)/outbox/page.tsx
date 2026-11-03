import type { Metadata } from 'next';

import { OutboxPage } from '@/features/operations';

export const metadata: Metadata = { title: 'Outbox' };

export default function Page() {
  return <OutboxPage />;
}
