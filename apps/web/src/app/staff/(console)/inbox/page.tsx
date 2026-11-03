import type { Metadata } from 'next';

import { InboxPage } from '@/features/inbox';

export const metadata: Metadata = { title: 'Inbox' };

export default function Page() {
  return <InboxPage />;
}
